package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-api/cmd/scheduler/di"
	"go-api/internal/infrastructure/config"
	"go-api/internal/infrastructure/observability"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// leaderLockKey is the pg advisory lock held by the single active scheduler.
const leaderLockKey int64 = 0x5c4ed01e

var (
	lagSeconds = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "scheduler_lag_seconds",
		Help: "Delay between the planned run time of the oldest due workflow and its dispatch.",
	})
	runsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "scheduler_runs_total",
		Help: "Runs handled by the scheduler, by outcome.",
	}, []string{"outcome"})
	tickDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "scheduler_tick_duration_seconds",
		Help:    "Duration of a dispatch tick, publication included.",
		Buckets: prometheus.DefBuckets,
	})
	isLeader = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "scheduler_leader",
		Help: "1 when this instance holds the scheduling lock.",
	})
)

func main() {
	logger := observability.NewLogger("scheduler")
	env := config.LoadScheduler()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	shutdownTracing, err := observability.SetupTracing(ctx, "scheduler")
	if err != nil {
		logger.Error("tracing setup failed", "error", err)
		os.Exit(1)
	}
	defer func() { _ = shutdownTracing(context.Background()) }()
	observability.ServeMetrics(ctx, env.MetricsAddr, logger)

	db := config.ConnectDatabaseURL(env.DatabaseURL)
	sqlDB, err := db.DB()
	if err != nil {
		logger.Error("database handle", "error", err)
		os.Exit(1)
	}
	container, err := di.NewContainer(db, env, logger)
	if err != nil {
		logger.Error("container", "error", err)
		os.Exit(1)
	}
	defer container.Producer.Close()

	s := &scheduler{container: container, env: env, logger: logger, http: &http.Client{Timeout: 10 * time.Second}}
	for ctx.Err() == nil {
		conn, err := acquireLeadership(ctx, sqlDB)
		if err != nil {
			if ctx.Err() == nil {
				logger.Warn("leadership not acquired, retrying", "error", err)
				sleep(ctx, 10*time.Second)
			}
			continue
		}
		logger.Info("scheduler is leader")
		isLeader.Set(1)
		s.lead(ctx, conn)
		isLeader.Set(0)
		_ = conn.Close()
	}
	logger.Info("scheduler stopped")
}

var errNotLeader = errors.New("another scheduler holds the lock")

// acquireLeadership takes the advisory lock on a dedicated connection; the
// lock lives as long as that connection.
func acquireLeadership(ctx context.Context, db *sql.DB) (*sql.Conn, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	var acquired bool
	if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", leaderLockKey).Scan(&acquired); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if !acquired {
		_ = conn.Close()
		return nil, errNotLeader
	}
	return conn, nil
}

type scheduler struct {
	container *di.Container
	env       *config.SchedulerConfig
	logger    *slog.Logger
	http      *http.Client
}

// lead ticks at every hh:mm:00 until ctx is done or the lock connection is lost.
func (s *scheduler) lead(ctx context.Context, lock *sql.Conn) {
	s.purgeExpiredRuns(ctx)
	for {
		now := time.Now().UTC()
		next := now.Truncate(time.Minute).Add(time.Minute)
		if !sleep(ctx, next.Sub(now)) {
			return
		}
		if err := lock.PingContext(ctx); err != nil {
			s.logger.Error("leadership lost", "error", err)
			return
		}
		s.tick(ctx, next)
	}
}

func (s *scheduler) tick(ctx context.Context, at time.Time) {
	started := time.Now()
	stats, err := s.container.Dispatch.Handle(ctx)
	tickDuration.Observe(time.Since(started).Seconds())
	if err != nil {
		s.logger.Error("dispatch failed", "error", err)
		return
	}
	lagSeconds.Set(stats.Lag.Seconds())
	runsTotal.WithLabelValues("dispatched").Add(float64(stats.Dispatched))
	runsTotal.WithLabelValues("skipped").Add(float64(stats.Skipped))
	runsTotal.WithLabelValues("timeout").Add(float64(stats.TimedOut))
	runsTotal.WithLabelValues("publish_failed").Add(float64(stats.PublishFailed))
	runsTotal.WithLabelValues("prepare_failed").Add(float64(stats.PrepareFailed))
	if stats.Dispatched+stats.Skipped+stats.TimedOut+stats.PublishFailed+stats.PrepareFailed > 0 {
		s.logger.Info("tick", "dispatched", stats.Dispatched, "skipped", stats.Skipped,
			"timed_out", stats.TimedOut, "publish_failed", stats.PublishFailed, "prepare_failed", stats.PrepareFailed, "lag_ms", stats.Lag.Milliseconds())
	}

	if at.Minute()%5 == 0 {
		if err := s.container.Maintain.ComputeAggregates(ctx); err != nil {
			s.logger.Error("aggregates failed", "error", err)
		}
	}
	if at.Minute() == 7 {
		s.purgeExpiredRuns(ctx)
	}
	s.heartbeat(ctx)
}

func (s *scheduler) purgeExpiredRuns(ctx context.Context) {
	if err := s.container.Maintain.PurgeExpiredRuns(ctx); err != nil {
		s.logger.Error("expired runs purge failed", "error", err)
	}
}

// heartbeat pings an external dead man's switch: no ping means no scheduling.
func (s *scheduler) heartbeat(ctx context.Context) {
	if s.env.HeartbeatURL == "" {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.env.HeartbeatURL, nil)
	if err != nil {
		return
	}
	resp, err := s.http.Do(req)
	if err != nil {
		s.logger.Warn("heartbeat failed", "error", err)
		return
	}
	_ = resp.Body.Close()
}

func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}
