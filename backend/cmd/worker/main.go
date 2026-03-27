package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/droptodrop/droptodrop/internal/config"
	"github.com/droptodrop/droptodrop/internal/database"
	"github.com/droptodrop/droptodrop/internal/jobs"
	"github.com/droptodrop/droptodrop/internal/logging"
	"github.com/droptodrop/droptodrop/internal/queue"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	logger := logging.New(cfg.Server.Env)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := database.Connect(ctx, cfg.Database)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer db.Close()

	redisClient, err := queue.NewClient(cfg.Redis, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to Redis")
	}
	defer redisClient.Close()

	worker := jobs.NewWorker(db, redisClient, cfg, logger)

	// Periodic cleanup of expired sessions and old webhook events
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := db.Exec(ctx, `DELETE FROM shop_sessions WHERE expires_at < NOW()`); err != nil {
					logger.Warn().Err(err).Msg("failed to cleanup expired sessions")
				} else {
					logger.Info().Msg("cleaned up expired sessions")
				}
				if _, err := db.Exec(ctx, `DELETE FROM webhook_events WHERE created_at < NOW() - INTERVAL '7 days'`); err != nil {
					logger.Warn().Err(err).Msg("failed to cleanup old webhook events")
				}
			}
		}
	}()

	// Handle shutdown
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		logger.Info().Msg("worker shutting down...")
		worker.Stop()
		cancel()
	}()

	logger.Info().Msg("worker starting...")
	worker.Start(ctx)
}
