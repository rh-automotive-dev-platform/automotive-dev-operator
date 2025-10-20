package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/rh-sdv-cloud-incubator/automotive-dev-operator/internal/buildapi"
)

func main() {
	// Parse command line flags
	var (
		kubeconfigPath = flag.String("kubeconfig-path", "", "Path to kubeconfig file")
		port           = flag.String("port", "", "Port to listen on (default: 8080)")
		namespace      = flag.String("namespace", "automotive-dev-operator-system", "Kubernetes namespace to use")
	)
	flag.Parse()

	// Set kubeconfig from flag if provided
	if *kubeconfigPath != "" {
		os.Setenv("KUBECONFIG", *kubeconfigPath)
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(handler))
	logger := logr.FromSlogHandler(handler)
	ctrl.SetLogger(logger)

	// Configure server address
	addr := ":8080"
	if *port != "" {
		addr = ":" + *port
	} else if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}

	if *namespace != "" {
		os.Setenv("BUILD_API_NAMESPACE", *namespace)
	}

	// Set Gin mode for development/testing
	if os.Getenv("GIN_MODE") == "" {
		os.Setenv("GIN_MODE", "debug")
	}

	slog.Info("starting build-api server",
		"addr", addr,
		"gin_mode", os.Getenv("GIN_MODE"),
		"kubeconfig", os.Getenv("KUBECONFIG"),
		"namespace", os.Getenv("BUILD_API_NAMESPACE"))

	apiServer := buildapi.NewAPIServer(addr, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		slog.Info("received shutdown signal")
		cancel()
	}()

	if err := apiServer.Start(ctx); err != nil {
		slog.Error("server error", "error", err)
	}
}
