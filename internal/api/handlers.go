package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	manager "github.com/Dhiraj1729/TargetServiceUpdate/internal/kubernetes"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

type Services interface {
	Create(context.Context, manager.CreateRequest) (*corev1.Service, error)
	UpdateTarget(context.Context, string, string) error
	Delete(context.Context, string) error
	Ready(context.Context) error
}

func New(services Services) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := services.Ready(ctx); err != nil {
			writeJSON(w, 503, map[string]string{"error": "Kubernetes API unavailable"})
			return
		}
		w.WriteHeader(200)
	})
	mux.HandleFunc("POST /api/v1/services", func(w http.ResponseWriter, r *http.Request) {
		var request manager.CreateRequest
		if !decode(w, r, &request) {
			return
		}
		svc, err := services.Create(r.Context(), request)
		if err != nil {
			respondError(w, err)
			return
		}
		writeJSON(w, 201, map[string]any{"serviceName": svc.Name, "namespace": svc.Namespace, "clusterIP": svc.Spec.ClusterIP, "podName": svc.Spec.Selector[manager.SelectorKey], "ports": request.Ports})
	})
	mux.HandleFunc("PUT /api/v1/services/{serviceName}/target", func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			PodName string `json:"podName"`
		}
		if !decode(w, r, &request) {
			return
		}
		if err := services.UpdateTarget(r.Context(), r.PathValue("serviceName"), request.PodName); err != nil {
			respondError(w, err)
			return
		}
		w.WriteHeader(204)
	})
	mux.HandleFunc("DELETE /api/v1/services/{serviceName}", func(w http.ResponseWriter, r *http.Request) {
		if err := services.Delete(r.Context(), r.PathValue("serviceName")); err != nil {
			respondError(w, err)
			return
		}
		w.WriteHeader(204)
	})
	return http.TimeoutHandler(mux, 15*time.Second, `{"error":"request timed out"}`)
}

func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	err := d.Decode(target)
	if err == nil {
		var extra any
		if next := d.Decode(&extra); next != io.EOF {
			err = next
			if err == nil {
				err = errors.New("expected one JSON object")
			}
		}
	}
	if err == nil {
		return true
	}
	code := 400
	var limit *http.MaxBytesError
	if errors.As(err, &limit) {
		code = 413
	}
	writeJSON(w, code, map[string]string{"error": "invalid JSON request: " + err.Error()})
	return false
}

func writeJSON(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}

func respondError(w http.ResponseWriter, err error) {
	code, message := 500, "Kubernetes operation failed"
	var rejected *manager.Error
	switch {
	case errors.As(err, &rejected):
		code, message = rejected.Code, rejected.Message
	case apierrors.IsNotFound(err):
		code, message = 404, "Service or target pod not found"
	case apierrors.IsAlreadyExists(err), apierrors.IsConflict(err):
		code, message = 409, "Service already exists or changed concurrently"
	case apierrors.IsInvalid(err), apierrors.IsBadRequest(err):
		code, message = 400, "Kubernetes rejected the resource"
	case apierrors.IsForbidden(err), apierrors.IsUnauthorized(err):
		code, message = 503, "Manager lacks Kubernetes permissions"
	case apierrors.IsTimeout(err), apierrors.IsServerTimeout(err), apierrors.IsServiceUnavailable(err), apierrors.IsTooManyRequests(err), errors.Is(err, context.DeadlineExceeded):
		code, message = 503, "Kubernetes API temporarily unavailable"
	}
	slog.Warn("request failed", "status", code, "error", err)
	writeJSON(w, code, map[string]string{"error": message})
}
