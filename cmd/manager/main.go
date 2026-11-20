package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Dhiraj1729/TargetServiceUpdate/internal/api"
	"github.com/Dhiraj1729/TargetServiceUpdate/internal/config"
	manager "github.com/Dhiraj1729/TargetServiceUpdate/internal/kubernetes"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("manager stopped", "error", err)
		os.Exit(1)
	}
}
func run() error {
	c, err := config.Load()
	if err != nil {
		return err
	}
	kube, err := rest.InClusterConfig()
	if errors.Is(err, rest.ErrNotInCluster) {
		kube, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{}).ClientConfig()
	}
	if err != nil {
		return err
	}
	kube.Timeout = 10 * time.Second
	client, err := coreclient.NewForConfig(kube)
	if err != nil {
		return err
	}
	svc := &manager.Manager{Client: client, Namespace: c.Namespace}
	server := &http.Server{Addr: c.Address, Handler: api.New(svc), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() { done <- server.ListenAndServe() }()
	slog.Info("manager listening", "address", c.Address, "namespace", c.Namespace)
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	}
}
