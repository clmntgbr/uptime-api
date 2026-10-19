package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"

	"go-api/cmd/worker-calls/di"
	"go-api/internal/infrastructure/config"
	"go-api/internal/infrastructure/observability"
)

func main() {
	logger := observability.NewLogger("worker-calls")
	env := config.LoadWorkerCalls()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	shutdownTracing, err := observability.SetupTracing(ctx, "worker-calls")
	if err != nil {
		logger.Error("tracing setup failed", "error", err)
		os.Exit(1)
	}
	defer func() { _ = shutdownTracing(context.Background()) }()
	observability.ServeMetrics(ctx, env.MetricsAddr, logger)

	container, err := di.NewContainer(env, logger)
	if err != nil {
		logger.Error("container", "error", err)
		os.Exit(1)
	}
	defer container.Producer.Close()

	logger.Info("worker-calls started", "concurrency", env.Concurrency)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return container.JobsConsumer.Run(gctx) })
	g.Go(func() error { return container.CancelsConsumer.Run(gctx) })
	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("worker-calls stopped with error", "error", err)
		os.Exit(1)
	}
}
