// Package app wires dependencies and manages the two network servers.
package app

import (
	"context"
	"errors"
	taskv1 "example.com/task-tracker-api/gen/task/v1"
	"example.com/task-tracker-api/internal/repository/memory"
	"example.com/task-tracker-api/internal/service"
	taskgrpc "example.com/task-tracker-api/internal/transport/grpc"
	"fmt"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
	"log/slog"
	"net"
	"net/http"
	"time"
)

type Config struct {
	GRPCAddr string
	HTTPAddr string
}

func Run(ctx context.Context, cfg Config) error {
	grpcListener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return fmt.Errorf("listen gRPC: %w", err)
	}
	defer grpcListener.Close()
	httpListener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listen HTTP: %w", err)
	}
	defer httpListener.Close()
	return Serve(ctx, grpcListener, httpListener)
}

// Serve also accepts ephemeral listeners, so integration tests use real network calls.
func Serve(ctx context.Context, grpcListener, httpListener net.Listener) error {
	repo := memory.NewTaskRepository()
	taskService := service.NewTaskService(repo)
	handler := taskgrpc.NewTaskHandler(taskService)
	grpcServer := grpc.NewServer()
	taskv1.RegisterTaskServiceServer(grpcServer, handler)

	// Dial loopback when the listener is bound to all interfaces.
	host, port, err := net.SplitHostPort(grpcListener.Addr().String())
	if err != nil {
		return err
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
		if ip.To4() != nil {
			host = "127.0.0.1"
		} else {
			host = "::1"
		}
	}
	conn, err := grpc.NewClient(net.JoinHostPort(host, port), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("create gateway channel: %w", err)
	}
	defer conn.Close()
	mux := runtime.NewServeMux(runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
		MarshalOptions: protojson.MarshalOptions{EmitUnpopulated: true},
	}))
	if err := taskv1.RegisterTaskServiceHandler(ctx, mux, conn); err != nil {
		return err
	}
	httpHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		mux.ServeHTTP(w, r.WithContext(requestCtx))
	})
	httpServer := &http.Server{
		Handler: httpHandler, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second,
	}
	errs := make(chan error, 2)
	go func() { errs <- grpcServer.Serve(grpcListener) }()
	go func() { errs <- httpServer.Serve(httpListener) }()
	slog.Info("task API started", "grpc", grpcListener.Addr(), "http", httpListener.Addr())

	var serveErr error
	select {
	case <-ctx.Done():
	case serveErr = <-errs:
	}
	// Stop accepting HTTP, drain gateway calls, then drain direct gRPC calls.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		_ = httpServer.Close()
	}
	done := make(chan struct{})
	go func() { grpcServer.GracefulStop(); close(done) }()
	select {
	case <-done:
	case <-shutdownCtx.Done():
		grpcServer.Stop()
		<-done
	}
	if errors.Is(serveErr, http.ErrServerClosed) || errors.Is(serveErr, grpc.ErrServerStopped) {
		return nil
	}
	return serveErr
}
