// Package kubernetes contains the manager's Kubernetes Service operations.
package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	SelectorKey = "statefulset.kubernetes.io/pod-name"
	ManagedBy   = "app.kubernetes.io/managed-by"
	ManagerName = "service-target-manager"
)

// Error represents a deliberate API rejection, as opposed to a client failure.
type Error struct {
	Code    int
	Message string
}

func (e *Error) Error() string              { return e.Message }
func reject(code int, message string) error { return &Error{code, message} }

type Port struct {
	Name       string `json:"name"`
	Port       int32  `json:"port"`
	TargetPort int32  `json:"targetPort"`
}
type CreateRequest struct {
	ServiceName string `json:"serviceName"`
	PodName     string `json:"podName"`
	Ports       []Port `json:"ports"`
}
type Manager struct {
	Client    coreclient.CoreV1Interface
	Namespace string
}

func serviceName(name string) error {
	if len(validation.IsDNS1035Label(name)) != 0 {
		return reject(400, "serviceName must be a valid DNS label starting with a letter (maximum 63 characters)")
	}
	return nil
}

func (m *Manager) target(ctx context.Context, name string) error {
	if len(validation.IsDNS1123Subdomain(name)) != 0 || len(name) > 63 {
		return reject(400, "podName must be a valid pod name of at most 63 characters")
	}
	pod, err := m.Client.Pods(m.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if pod.Labels[SelectorKey] != name {
		return reject(422, "target pod must have the StatefulSet pod-name label matching podName")
	}
	if pod.DeletionTimestamp != nil {
		return reject(422, "target pod is terminating")
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return nil
		}
	}
	return reject(422, "target pod is not Ready")
}

func (m *Manager) Create(ctx context.Context, request CreateRequest) (*corev1.Service, error) {
	if err := serviceName(request.ServiceName); err != nil {
		return nil, err
	}
	if len(request.Ports) == 0 {
		return nil, reject(400, "at least one port is required")
	}
	names, numbers := map[string]bool{}, map[int32]bool{}
	ports := make([]corev1.ServicePort, 0, len(request.Ports))
	for _, p := range request.Ports {
		if len(validation.IsValidPortName(p.Name)) != 0 || names[p.Name] || numbers[p.Port] || p.Port < 1 || p.Port > 65535 || p.TargetPort < 1 || p.TargetPort > 65535 {
			return nil, reject(400, "ports require unique valid names and port numbers; port and targetPort must be 1-65535")
		}
		names[p.Name], numbers[p.Port] = true, true
		ports = append(ports, corev1.ServicePort{Name: p.Name, Port: p.Port, TargetPort: intstr.FromInt32(p.TargetPort), Protocol: corev1.ProtocolTCP})
	}
	if err := m.target(ctx, request.PodName); err != nil {
		return nil, err
	}
	return m.Client.Services(m.Namespace).Create(ctx, &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: request.ServiceName, Namespace: m.Namespace, Labels: map[string]string{ManagedBy: ManagerName}},
		Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP, Selector: map[string]string{SelectorKey: request.PodName}, Ports: ports},
	}, metav1.CreateOptions{})
}

func (m *Manager) managed(ctx context.Context, name string) (*corev1.Service, error) {
	if err := serviceName(name); err != nil {
		return nil, err
	}
	svc, err := m.Client.Services(m.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if svc.Labels[ManagedBy] != ManagerName {
		return nil, reject(403, "Service is not managed by service-target-manager")
	}
	if svc.Spec.Type != corev1.ServiceTypeClusterIP || svc.Spec.ClusterIP == corev1.ClusterIPNone {
		return nil, reject(422, "only non-headless ClusterIP Services are supported")
	}
	return svc, nil
}

func (m *Manager) UpdateTarget(ctx context.Context, name, pod string) error {
	svc, err := m.managed(ctx, name)
	if err != nil {
		return err
	}
	if err := m.target(ctx, pod); err != nil {
		return err
	}
	// Null out old keys: JSON merge-patch otherwise retains stale selector keys.
	selector := map[string]any{}
	for key := range svc.Spec.Selector {
		selector[key] = nil
	}
	selector[SelectorKey] = pod
	patch, err := json.Marshal(map[string]any{
		"metadata": map[string]any{"resourceVersion": svc.ResourceVersion},
		"spec":     map[string]any{"selector": selector},
	})
	if err != nil {
		return err
	}
	// resourceVersion rejects concurrent changes; clients receive 409 and can
	// reconsider their activation decision. Do not blindly retry stale decisions.
	_, err = m.Client.Services(m.Namespace).Patch(ctx, name, types.MergePatchType, patch, metav1.PatchOptions{})
	return err
}

func (m *Manager) Delete(ctx context.Context, name string) error {
	svc, err := m.managed(ctx, name)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	err = m.Client.Services(m.Namespace).Delete(ctx, name, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &svc.UID, ResourceVersion: &svc.ResourceVersion}})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (m *Manager) Ready(ctx context.Context) error {
	_, err := m.Client.Services(m.Namespace).List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return fmt.Errorf("Kubernetes access: %w", err)
	}
	return nil
}
