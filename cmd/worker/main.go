package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"go-api/cmd/worker/di"
	"go-api/internal/infrastructure/config"
	"go-api/internal/infrastructure/observability"
	"go-api/internal/infrastructure/persistence/schema"

	"golang.org/x/sync/errgroup"
)

func main() {
	logger := observability.NewLogger("worker")
	env := config.Load()
	db := config.ConnectDatabase(env)

	if err := schema.AssertModelsMatchDB(db); err != nil {
		log.Fatalf("schema check failed: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	shutdownTracing, err := observability.SetupTracing(ctx, "worker")
	if err != nil {
		log.Fatalf("tracing setup failed: %v", err)
	}
	defer func() { _ = shutdownTracing(context.Background()) }()
	observability.ServeMetrics(ctx, env.MetricsAddr, logger)

	container := di.NewContainer(db, env)
	defer container.Producer.Close()

	go container.Relay.Start(ctx)

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return container.EventsConsumer.Run(gctx) })
	g.Go(func() error { return container.ResultsConsumer.Run(gctx) })
	if err := g.Wait(); err != nil {
		log.Fatalf("worker stopped with error: %v", err)
	}
}
