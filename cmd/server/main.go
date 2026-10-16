package main

import (
	"context"
	"example.com/task-tracker-api/internal/app"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	grpcAddr := flag.String("grpc-addr", "127.0.0.1:50051", "gRPC listen address")
	httpAddr := flag.String("http-addr", "127.0.0.1:8080", "HTTP Gateway listen address")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.Run(ctx, app.Config{GRPCAddr: *grpcAddr, HTTPAddr: *httpAddr}); err != nil {
		log.Fatal(err)
	}
}
