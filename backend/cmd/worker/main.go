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
	"github.com/droptodrop/droptodrop/internal/trust"
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
	trustSvc := trust.NewService(db, logger)

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

				// Enqueue inventory sync for all active imports
				rows, err := db.Query(ctx, `
					SELECT ri.id, ri.reseller_shop_id
					FROM reseller_imports ri
					WHERE ri.status = 'active' AND ri.shopify_product_id IS NOT NULL
				`)
				if err == nil {
					syncCount := 0
					for rows.Next() {
						var importID, resellerShopID string
						rows.Scan(&importID, &resellerShopID)
						redisClient.Enqueue(ctx, "imports", "sync_product", map[string]string{
							"import_id":        importID,
							"reseller_shop_id": resellerShopID,
						}, 1)
						syncCount++
					}
					rows.Close()
					if syncCount > 0 {
						logger.Info().Int("count", syncCount).Msg("scheduled inventory sync for active imports")
					}
				}

				// Update platform stats
				db.Exec(ctx, `
					UPDATE platform_stats SET
						total_products = (SELECT COUNT(*) FROM supplier_listings WHERE status = 'active'),
						total_orders = (SELECT COUNT(*) FROM routed_orders),
						total_suppliers = (SELECT COUNT(*) FROM shops WHERE role = 'supplier' AND status = 'active'),
						total_resellers = (SELECT COUNT(*) FROM shops WHERE role = 'reseller' AND status = 'active'),
						total_revenue = COALESCE((SELECT SUM(total_wholesale_amount) FROM routed_orders WHERE status = 'fulfilled'), 0),
						updated_at = NOW()
					WHERE id = 1
				`)

				// Recalculate supplier trust scores
				supplierRows, err := db.Query(ctx, `SELECT id FROM shops WHERE role = 'supplier' AND status = 'active'`)
				if err == nil {
					for supplierRows.Next() {
						var supplierID string
						supplierRows.Scan(&supplierID)
						trustSvc.RecalculateStats(ctx, supplierID)
					}
					supplierRows.Close()
					logger.Info().Msg("recalculated supplier trust scores")
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
