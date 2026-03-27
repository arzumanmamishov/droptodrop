package main

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/droptodrop/droptodrop/internal/audit"
	authpkg "github.com/droptodrop/droptodrop/internal/auth"
	"github.com/droptodrop/droptodrop/internal/billing"
	"github.com/droptodrop/droptodrop/internal/compliance"
	"github.com/droptodrop/droptodrop/internal/config"
	"github.com/droptodrop/droptodrop/internal/database"
	"github.com/droptodrop/droptodrop/internal/fulfillments"
	"github.com/droptodrop/droptodrop/internal/health"
	"github.com/droptodrop/droptodrop/internal/imports"
	"github.com/droptodrop/droptodrop/internal/logging"
	"github.com/droptodrop/droptodrop/internal/middleware"
	"github.com/droptodrop/droptodrop/internal/orders"
	"github.com/droptodrop/droptodrop/internal/products"
	"github.com/droptodrop/droptodrop/internal/queue"
	"github.com/droptodrop/droptodrop/internal/shops"
	"github.com/droptodrop/droptodrop/internal/webhooks"
	"github.com/droptodrop/droptodrop/pkg/shopify"
)

func main() {
	// Load .env in development
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	logger := logging.New(cfg.Server.Env)

	// Connect to database
	ctx := context.Background()
	db, err := database.Connect(ctx, cfg.Database)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer db.Close()

	// Connect to Redis
	redisClient, err := queue.NewClient(cfg.Redis, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to Redis")
	}
	defer redisClient.Close()

	// Initialize services
	auditSvc := audit.NewService(db, logger)
	shopsSvc := shops.NewService(db, logger, auditSvc)
	productsSvc := products.NewService(db, logger, auditSvc)
	importsSvc := imports.NewService(db, redisClient, logger, auditSvc)
	ordersSvc := orders.NewService(db, redisClient, logger, auditSvc)
	fulfillmentsSvc := fulfillments.NewService(db, redisClient, logger, auditSvc)

	// Initialize handlers
	authHandler := authpkg.NewHandler(db, cfg.Shopify, cfg.Session, cfg.Security.EncryptionKey, logger, auditSvc)
	healthHandler := health.NewHandler(db, redisClient)
	webhookHandler := webhooks.NewHandler(db, redisClient, shopsSvc, ordersSvc, cfg.Shopify.APISecret, logger, auditSvc)
	complianceHandler := compliance.NewHandler(db, cfg.Shopify.APISecret, logger, auditSvc)
	billingHandler := billing.NewHandler(db, logger)

	// Setup Gin
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestLogger(logger))
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.CORS(cfg.Shopify.AppURL))

	// ===== Public routes (no auth) =====

	// Health
	r.GET("/health", healthHandler.Liveness)
	r.GET("/health/ready", healthHandler.Readiness)

	// OAuth
	r.GET("/auth/install", authHandler.Install)
	r.GET("/auth/callback", authHandler.Callback)

	// Webhooks (HMAC verified internally)
	wh := r.Group("/webhooks")
	{
		wh.POST("/app/uninstalled", webhookHandler.AppUninstalled)
		wh.POST("/orders/create", webhookHandler.OrdersCreate)
		wh.POST("/fulfillments/create", webhookHandler.FulfillmentsCreate)
		wh.POST("/products/update", webhookHandler.ProductsUpdate)
		wh.POST("/products/delete", webhookHandler.ProductsDelete)
		wh.POST("/inventory/update", webhookHandler.InventoryUpdate)
	}

	// Compliance webhooks
	comp := r.Group("/webhooks/compliance")
	{
		comp.POST("/customers-data-request", complianceHandler.CustomersDataRequest)
		comp.POST("/customers-redact", complianceHandler.CustomersRedact)
		comp.POST("/shop-redact", complianceHandler.ShopRedact)
	}

	// ===== Authenticated API routes =====
	api := r.Group("/api/v1")
	api.Use(middleware.SessionAuth(db, cfg.Shopify.APIKey, cfg.Shopify.APISecret, cfg.Session.MaxAge, logger))
	api.Use(middleware.RateLimit(cfg.Security.RateLimitRPS, cfg.Security.RateLimitBurst))
	{
		// Shop
		api.GET("/shop", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			logger.Info().Str("shop_id", shopID.(string)).Msg("fetching shop")
			shop, err := shopsSvc.GetByID(c.Request.Context(), shopID.(string))
			if err != nil {
				logger.Error().Err(err).Str("shop_id", shopID.(string)).Msg("shop not found")
				c.JSON(http.StatusNotFound, gin.H{"error": "shop not found"})
				return
			}
			c.JSON(http.StatusOK, shop)
		})

		api.POST("/shop/role", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			var body struct {
				Role string `json:"role" binding:"required,oneof=supplier reseller"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if err := shopsSvc.SetRole(c.Request.Context(), shopID.(string), body.Role); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "ok", "role": body.Role})
		})

		// Dashboard
		api.GET("/dashboard", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			role, _ := c.Get("shop_role")
			sid := shopID.(string)

			dashboard := gin.H{"role": role}

			if role == "supplier" {
				listings, total, _ := productsSvc.ListSupplierListings(c.Request.Context(), sid, "active", 100, 0)
				orders, orderTotal, _ := ordersSvc.ListRoutedOrders(c.Request.Context(), sid, "supplier", "", 10, 0)
				draftListings, draftTotal, _ := productsSvc.ListSupplierListings(c.Request.Context(), sid, "draft", 100, 0)
				_ = draftListings
				dashboard["active_listings"] = total
				dashboard["draft_listings"] = draftTotal
				dashboard["listings_preview"] = listings
				dashboard["recent_orders"] = orders
				dashboard["order_count"] = orderTotal
				// Count pending orders
				pendingOrders, pendingTotal, _ := ordersSvc.ListRoutedOrders(c.Request.Context(), sid, "supplier", "pending", 100, 0)
				_ = pendingOrders
				dashboard["pending_order_count"] = pendingTotal
			} else if role == "reseller" {
				imports, importTotal, _ := importsSvc.List(c.Request.Context(), sid, 100, 0)
				orders, orderTotal, _ := ordersSvc.ListRoutedOrders(c.Request.Context(), sid, "reseller", "", 10, 0)
				dashboard["imported_products"] = importTotal
				dashboard["imports_preview"] = imports
				dashboard["recent_orders"] = orders
				dashboard["order_count"] = orderTotal
				pendingOrders, pendingTotal, _ := ordersSvc.ListRoutedOrders(c.Request.Context(), sid, "reseller", "pending", 100, 0)
				_ = pendingOrders
				dashboard["pending_order_count"] = pendingTotal
			}

			c.JSON(http.StatusOK, dashboard)
		})

		// ===== Supplier routes =====
		supplier := api.Group("/supplier")
		supplier.Use(middleware.RequireRole("supplier"))
		{
			supplier.GET("/profile", func(c *gin.Context) {
				shopID, _ := c.Get("shop_id")
				profile, err := shopsSvc.GetSupplierProfile(c.Request.Context(), shopID.(string))
				if err != nil {
					c.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
					return
				}
				c.JSON(http.StatusOK, profile)
			})

			supplier.PUT("/profile", func(c *gin.Context) {
				shopID, _ := c.Get("shop_id")
				var body map[string]interface{}
				if err := c.ShouldBindJSON(&body); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				if err := shopsSvc.UpdateSupplierProfile(c.Request.Context(), shopID.(string), body); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})

			supplier.GET("/listings", func(c *gin.Context) {
				shopID, _ := c.Get("shop_id")
				status := c.Query("status")
				limit := getIntQuery(c, "limit", 20)
				offset := getIntQuery(c, "offset", 0)
				listings, total, err := productsSvc.ListSupplierListings(c.Request.Context(), shopID.(string), status, limit, offset)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, gin.H{"listings": listings, "total": total})
			})

			// Fetch products from the supplier's Shopify store (for Resource Picker)
			supplier.GET("/shop-products", func(c *gin.Context) {
				shopID, _ := c.Get("shop_id")
				sid := shopID.(string)

				// Get shop credentials
				var shopDomain, encryptedToken string
				err := db.QueryRow(c.Request.Context(), `
					SELECT s.shopify_domain, ai.access_token
					FROM shops s
					JOIN app_installations ai ON ai.shop_id = s.id AND ai.is_active = TRUE
					WHERE s.id = $1
				`, sid).Scan(&shopDomain, &encryptedToken)
				if err != nil {
					logger.Error().Err(err).Str("shop_id", sid).Msg("shop credentials not found")
					c.JSON(http.StatusInternalServerError, gin.H{"error": "shop credentials not found"})
					return
				}

				token, err := authpkg.Decrypt(encryptedToken, cfg.Security.EncryptionKey)
				if err != nil {
					logger.Error().Err(err).Str("shop", shopDomain).Msg("failed to decrypt token")
					c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to decrypt credentials"})
					return
				}

				client := shopify.NewClient(shopDomain, token, logger)
				cursor := c.Query("cursor")
				prods, nextCursor, err := products.FetchShopProducts(c.Request.Context(), client, logger, cursor, 20)
				if err != nil {
					logger.Error().Err(err).Str("shop", shopDomain).Msg("failed to fetch shop products")
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}

				c.JSON(http.StatusOK, gin.H{
					"products":    prods,
					"next_cursor": nextCursor,
				})
			})

			supplier.POST("/listings", func(c *gin.Context) {
				shopID, _ := c.Get("shop_id")
				var input products.CreateListingInput
				if err := c.ShouldBindJSON(&input); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				listing, err := productsSvc.CreateListing(c.Request.Context(), shopID.(string), input)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusCreated, listing)
			})

			supplier.GET("/listings/:id", func(c *gin.Context) {
				listing, err := productsSvc.GetListing(c.Request.Context(), c.Param("id"))
				if err != nil {
					c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, listing)
			})

			supplier.PUT("/listings/:id", func(c *gin.Context) {
				shopID, _ := c.Get("shop_id")
				var input products.UpdateListingInput
				if err := c.ShouldBindJSON(&input); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				if err := productsSvc.UpdateListing(c.Request.Context(), shopID.(string), c.Param("id"), input); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})

			supplier.PUT("/listings/:id/status", func(c *gin.Context) {
				shopID, _ := c.Get("shop_id")
				var body struct {
					Status string `json:"status" binding:"required,oneof=draft active paused archived"`
				}
				if err := c.ShouldBindJSON(&body); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				if err := productsSvc.UpdateListingStatus(c.Request.Context(), shopID.(string), c.Param("id"), body.Status); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})

			supplier.DELETE("/listings/:id", func(c *gin.Context) {
				shopID, _ := c.Get("shop_id")
				if err := productsSvc.DeleteListing(c.Request.Context(), shopID.(string), c.Param("id")); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})

			supplier.GET("/orders", func(c *gin.Context) {
				shopID, _ := c.Get("shop_id")
				status := c.Query("status")
				limit := getIntQuery(c, "limit", 20)
				offset := getIntQuery(c, "offset", 0)
				ords, total, err := ordersSvc.ListRoutedOrders(c.Request.Context(), shopID.(string), "supplier", status, limit, offset)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, gin.H{"orders": ords, "total": total})
			})

			supplier.POST("/orders/:id/accept", func(c *gin.Context) {
				shopID, _ := c.Get("shop_id")
				if err := ordersSvc.AcceptOrder(c.Request.Context(), c.Param("id"), shopID.(string)); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})

			supplier.POST("/orders/:id/reject", func(c *gin.Context) {
				shopID, _ := c.Get("shop_id")
				var body struct {
					Reason string `json:"reason"`
				}
				c.ShouldBindJSON(&body)
				if err := ordersSvc.RejectOrder(c.Request.Context(), c.Param("id"), shopID.(string), body.Reason); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})

			supplier.POST("/orders/:id/fulfill", func(c *gin.Context) {
				shopID, _ := c.Get("shop_id")
				var input fulfillments.AddTrackingInput
				if err := c.ShouldBindJSON(&input); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				input.RoutedOrderID = c.Param("id")
				event, err := fulfillmentsSvc.AddTracking(c.Request.Context(), shopID.(string), input)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, event)
			})
		}

		// ===== Reseller routes =====
		reseller := api.Group("/reseller")
		reseller.Use(middleware.RequireRole("reseller"))
		{
			reseller.GET("/profile", func(c *gin.Context) {
				shopID, _ := c.Get("shop_id")
				profile, err := shopsSvc.GetResellerProfile(c.Request.Context(), shopID.(string))
				if err != nil {
					c.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
					return
				}
				c.JSON(http.StatusOK, profile)
			})

			reseller.GET("/marketplace", func(c *gin.Context) {
				var filters products.MarketplaceFilters
				c.ShouldBindQuery(&filters)
				limit := getIntQuery(c, "limit", 20)
				offset := getIntQuery(c, "offset", 0)
				listings, total, err := productsSvc.ListMarketplace(c.Request.Context(), filters, limit, offset)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, gin.H{"listings": listings, "total": total})
			})

			reseller.GET("/marketplace/:id", func(c *gin.Context) {
				listing, err := productsSvc.GetListing(c.Request.Context(), c.Param("id"))
				if err != nil {
					c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, listing)
			})

			reseller.POST("/imports", func(c *gin.Context) {
				shopID, _ := c.Get("shop_id")
				var input imports.ImportInput
				if err := c.ShouldBindJSON(&input); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				imp, err := importsSvc.Create(c.Request.Context(), shopID.(string), input)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusCreated, imp)
			})

			reseller.GET("/imports", func(c *gin.Context) {
				shopID, _ := c.Get("shop_id")
				limit := getIntQuery(c, "limit", 20)
				offset := getIntQuery(c, "offset", 0)
				imps, total, err := importsSvc.List(c.Request.Context(), shopID.(string), limit, offset)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, gin.H{"imports": imps, "total": total})
			})

			reseller.POST("/imports/:id/resync", func(c *gin.Context) {
				shopID, _ := c.Get("shop_id")
				if err := importsSvc.ResyncImport(c.Request.Context(), shopID.(string), c.Param("id")); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})

			reseller.GET("/orders", func(c *gin.Context) {
				shopID, _ := c.Get("shop_id")
				status := c.Query("status")
				limit := getIntQuery(c, "limit", 20)
				offset := getIntQuery(c, "offset", 0)
				ords, total, err := ordersSvc.ListRoutedOrders(c.Request.Context(), shopID.(string), "reseller", status, limit, offset)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, gin.H{"orders": ords, "total": total})
			})
		}

		// ===== Shared routes (both roles) =====
		api.GET("/orders/:id", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			order, err := ordersSvc.GetRoutedOrder(c.Request.Context(), c.Param("id"), shopID.(string))
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}
			// Include fulfillment events
			events, _ := fulfillmentsSvc.ListByOrder(c.Request.Context(), c.Param("id"))
			c.JSON(http.StatusOK, gin.H{"order": order, "fulfillments": events})
		})

		// Settings
		api.GET("/settings", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			var settings struct {
				NotificationsEnabled bool   `json:"notifications_enabled"`
				NotificationEmail    string `json:"notification_email"`
				SupportEmail         string `json:"support_email"`
				PrivacyPolicyURL     string `json:"privacy_policy_url"`
				TermsURL             string `json:"terms_url"`
				DataRetentionDays    int    `json:"data_retention_days"`
				BillingPlan          string `json:"billing_plan"`
			}
			err := db.QueryRow(c.Request.Context(), `
				SELECT notifications_enabled, COALESCE(notification_email,''), COALESCE(support_email,''),
					COALESCE(privacy_policy_url,''), COALESCE(terms_url,''), data_retention_days, COALESCE(billing_plan,'free')
				FROM app_settings WHERE shop_id = $1
			`, shopID).Scan(&settings.NotificationsEnabled, &settings.NotificationEmail, &settings.SupportEmail,
				&settings.PrivacyPolicyURL, &settings.TermsURL, &settings.DataRetentionDays, &settings.BillingPlan)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "settings not found"})
				return
			}
			c.JSON(http.StatusOK, settings)
		})

		api.PUT("/settings", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			var body map[string]interface{}
			if err := c.ShouldBindJSON(&body); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			_, err := db.Exec(c.Request.Context(), `
				UPDATE app_settings SET
					notifications_enabled = COALESCE($2, notifications_enabled),
					notification_email = COALESCE($3, notification_email),
					support_email = COALESCE($4, support_email),
					privacy_policy_url = COALESCE($5, privacy_policy_url),
					terms_url = COALESCE($6, terms_url),
					data_retention_days = COALESCE($7, data_retention_days)
				WHERE shop_id = $1
			`, shopID,
				body["notifications_enabled"],
				body["notification_email"],
				body["support_email"],
				body["privacy_policy_url"],
				body["terms_url"],
				body["data_retention_days"],
			)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			auditSvc.Log(c.Request.Context(), shopID.(string), "merchant", shopID.(string), "settings_updated", "app_settings", shopID.(string), body, "success", "")
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		// Billing
		api.GET("/billing", billingHandler.GetStatus)

		// Audit logs
		api.GET("/audit", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			limit := getIntQuery(c, "limit", 50)
			offset := getIntQuery(c, "offset", 0)
			entries, total, err := auditSvc.List(c.Request.Context(), shopID.(string), limit, offset)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"entries": entries, "total": total})
		})
	}

	// Serve frontend static files if available (production single-container mode)
	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" {
		staticDir = "static"
	}
	if info, err := os.Stat(staticDir); err == nil && info.IsDir() {
		r.NoRoute(func(c *gin.Context) {
			path := c.Request.URL.Path
			// Serve static files directly if they exist
			if !strings.HasPrefix(path, "/api/") && !strings.HasPrefix(path, "/auth/") &&
				!strings.HasPrefix(path, "/webhooks/") && !strings.HasPrefix(path, "/health") {
				fullPath := filepath.Join(staticDir, path)
				if _, err := fs.Stat(os.DirFS(staticDir), strings.TrimPrefix(path, "/")); err == nil {
					c.File(fullPath)
					return
				}
				// SPA fallback: serve index.html for unmatched routes
				c.File(filepath.Join(staticDir, "index.html"))
				return
			}
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		})
		logger.Info().Str("dir", staticDir).Msg("serving frontend static files")
	} else {
		// Fallback: serve minimal Shopify App Bridge page when no frontend build exists
		r.NoRoute(func(c *gin.Context) {
			path := c.Request.URL.Path
			if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/auth/") ||
				strings.HasPrefix(path, "/webhooks/") || strings.HasPrefix(path, "/health") {
				c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
				return
			}
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.String(http.StatusOK, `<!DOCTYPE html>
<html><head>
<meta name="shopify-api-key" content="`+cfg.Shopify.APIKey+`" />
<script src="https://cdn.shopify.com/shopifycloud/app-bridge.js"></script>
<style>body{font-family:-apple-system,BlinkMacSystemFont,Segoe UI,Roboto,sans-serif;display:flex;justify-content:center;align-items:center;height:100vh;margin:0;background:#f6f6f7}
.card{background:#fff;border-radius:12px;padding:40px;text-align:center;box-shadow:0 1px 3px rgba(0,0,0,.08)}</style>
</head><body><div class="card"><h1>DropToDrop</h1><p>Shopify Dropshipping Network</p><p>App is running. Frontend deployment pending.</p></div></body></html>`)
		})
	}

	// Start server
	addr := fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		logger.Info().Str("addr", addr).Msg("server starting")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("server failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Fatal().Err(err).Msg("server forced to shutdown")
	}

	logger.Info().Msg("server stopped")
}

func getIntQuery(c *gin.Context, key string, def int) int {
	val := c.Query(key)
	if val == "" {
		return def
	}
	var i int
	fmt.Sscanf(val, "%d", &i)
	if i <= 0 {
		return def
	}
	return i
}
