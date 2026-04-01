package main

import (
	"context"
	"encoding/json"
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
	"github.com/droptodrop/droptodrop/internal/disputes"
	"github.com/droptodrop/droptodrop/internal/fulfillments"
	"github.com/droptodrop/droptodrop/internal/health"
	"github.com/droptodrop/droptodrop/internal/imports"
	"github.com/droptodrop/droptodrop/internal/inappnotif"
	"github.com/droptodrop/droptodrop/internal/logging"
	"github.com/droptodrop/droptodrop/internal/middleware"
	"github.com/droptodrop/droptodrop/internal/orders"
	"github.com/droptodrop/droptodrop/internal/products"
	"github.com/droptodrop/droptodrop/internal/queue"
	"github.com/droptodrop/droptodrop/internal/shops"
	"github.com/droptodrop/droptodrop/internal/advanced"
	"github.com/droptodrop/droptodrop/internal/messaging"
	"github.com/droptodrop/droptodrop/internal/trust"
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

	// Connect to Redis (optional — falls back to in-memory processing)
	redisClient, err := queue.NewClient(cfg.Redis, logger)
	if err != nil {
		logger.Warn().Err(err).Msg("Redis not available, using in-memory job processing")
		redisClient = queue.NewFallbackClient(logger)
	}
	defer redisClient.Close()

	// Initialize services
	auditSvc := audit.NewService(db, logger)
	shopsSvc := shops.NewService(db, logger, auditSvc)
	productsSvc := products.NewService(db, logger, auditSvc)
	importsSvc := imports.NewService(db, redisClient, logger, auditSvc)
	imports.SetEncryptionKey(cfg.Security.EncryptionKey)
	ordersSvc := orders.NewService(db, redisClient, logger, auditSvc)
	fulfillmentsSvc := fulfillments.NewService(db, redisClient, logger, auditSvc)
	disputesSvc := disputes.NewService(db, logger)
	inappNotifSvc := inappnotif.NewService(db, logger)

	// Initialize handlers
	authHandler := authpkg.NewHandler(db, cfg.Shopify, cfg.Session, cfg.Security.EncryptionKey, logger, auditSvc)
	healthHandler := health.NewHandler(db, redisClient)
	webhookHandler := webhooks.NewHandler(db, redisClient, shopsSvc, ordersSvc, cfg.Shopify.APISecret, logger, auditSvc)
	complianceHandler := compliance.NewHandler(db, cfg.Shopify.APISecret, logger, auditSvc)
	billingHandler := billing.NewHandler(db, logger)
	trustSvc := trust.NewService(db, logger)
	msgSvc := messaging.NewService(db, logger)
	advSvc := advanced.NewService(db, logger)

	// Setup Gin
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.MaxMultipartMemory = 8 << 20 // 8 MB
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

	// Legal pages (public, no auth)
	r.GET("/privacy", func(c *gin.Context) {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, privacyPolicyHTML)
	})
	r.GET("/terms", func(c *gin.Context) {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, termsOfServiceHTML)
	})

	// Public API (no auth) — for supplier directory and platform stats
	pub := r.Group("/public")
	{
		pub.GET("/stats", func(c *gin.Context) {
			var totalProducts, totalOrders, totalSuppliers, totalResellers int
			db.QueryRow(c.Request.Context(), `SELECT COUNT(*) FROM supplier_listings WHERE status = 'active'`).Scan(&totalProducts)
			db.QueryRow(c.Request.Context(), `SELECT COUNT(*) FROM routed_orders`).Scan(&totalOrders)
			db.QueryRow(c.Request.Context(), `SELECT COUNT(*) FROM shops WHERE role = 'supplier' AND status = 'active'`).Scan(&totalSuppliers)
			db.QueryRow(c.Request.Context(), `SELECT COUNT(*) FROM shops WHERE role = 'reseller' AND status = 'active'`).Scan(&totalResellers)
			c.JSON(http.StatusOK, gin.H{
				"total_products":  totalProducts,
				"total_orders":    totalOrders,
				"total_suppliers": totalSuppliers,
				"total_resellers": totalResellers,
			})
		})

		pub.GET("/suppliers", func(c *gin.Context) {
			rows, err := db.Query(c.Request.Context(), `
				SELECT s.id, s.shopify_domain, COALESCE(sp.company_name, s.shopify_domain),
					COALESCE(sp.default_processing_days, 3), COALESCE(sp.is_verified, FALSE),
					(SELECT COUNT(*) FROM supplier_listings sl WHERE sl.supplier_shop_id = s.id AND sl.status = 'active')
				FROM shops s
				LEFT JOIN supplier_profiles sp ON sp.shop_id = s.id
				WHERE s.role = 'supplier' AND s.status = 'active'
				ORDER BY (SELECT COUNT(*) FROM supplier_listings sl WHERE sl.supplier_shop_id = s.id AND sl.status = 'active') DESC
				LIMIT 50
			`)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer rows.Close()
			var suppliers []gin.H
			for rows.Next() {
				var id, domain, name string
				var days int
				var verified bool
				var count int
				rows.Scan(&id, &domain, &name, &days, &verified, &count)
				suppliers = append(suppliers, gin.H{
					"id": id, "domain": domain, "name": name,
					"processing_days": days, "verified": verified, "listing_count": count,
				})
			}
			c.JSON(http.StatusOK, gin.H{"suppliers": suppliers})
		})
	}

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
			shopDomain, _ := c.Get("shop_domain")
			shop, err := shopsSvc.GetByID(c.Request.Context(), shopID.(string))
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "shop not found"})
				return
			}
			// Register webhooks if not already registered (non-destructive)
			go func() {
				var encToken string
				db.QueryRow(context.Background(), `SELECT access_token FROM app_installations WHERE shop_id = $1 AND is_active = TRUE`, shopID.(string)).Scan(&encToken)
				if encToken != "" {
					token, err := authpkg.Decrypt(encToken, cfg.Security.EncryptionKey)
					if err == nil {
						domain := ""
						if d, ok := shopDomain.(string); ok { domain = d }
						if domain == "" { domain = shop.ShopifyDomain }
						client := shopify.NewClient(domain, token, logger)
						bgCtx := context.Background()
						for _, topic := range []string{"ORDERS_CREATE", "FULFILLMENTS_CREATE", "PRODUCTS_DELETE", "PRODUCTS_UPDATE", "APP_UNINSTALLED"} {
							paths := map[string]string{
								"ORDERS_CREATE":      "/webhooks/orders/create",
								"FULFILLMENTS_CREATE": "/webhooks/fulfillments/create",
								"PRODUCTS_DELETE":     "/webhooks/products/delete",
								"PRODUCTS_UPDATE":     "/webhooks/products/update",
								"APP_UNINSTALLED":     "/webhooks/app/uninstalled",
							}
							// Use RegisterWebhook (not DeleteAndRegister) to avoid gaps
							client.RegisterWebhook(bgCtx, topic, cfg.Shopify.AppURL+paths[topic])
						}
					}
				}
			}()
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

			supplier.DELETE("/listings/:id", func(c *gin.Context) {
				shopID, _ := c.Get("shop_id")
				if err := productsSvc.DeleteListing(c.Request.Context(), shopID.(string), c.Param("id")); err != nil {
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

			// Reseller management for suppliers
			supplier.GET("/resellers", func(c *gin.Context) {
				shopID, _ := c.Get("shop_id")
				sid := shopID.(string)
				rows, err := db.Query(c.Request.Context(), `
					SELECT DISTINCT ri.reseller_shop_id, s.shopify_domain,
						COALESCE(srl.status, 'active') as link_status,
						COALESCE(srl.reason, '') as reason,
						COUNT(ri.id) as import_count,
						(SELECT COUNT(*) FROM routed_orders ro WHERE ro.reseller_shop_id = ri.reseller_shop_id AND ro.supplier_shop_id = $1) as order_count
					FROM reseller_imports ri
					JOIN supplier_listings sl ON sl.id = ri.supplier_listing_id AND sl.supplier_shop_id = $1
					JOIN shops s ON s.id = ri.reseller_shop_id
					LEFT JOIN supplier_reseller_links srl ON srl.supplier_shop_id = $1 AND srl.reseller_shop_id = ri.reseller_shop_id
					GROUP BY ri.reseller_shop_id, s.shopify_domain, srl.status, srl.reason
					ORDER BY order_count DESC
				`, sid)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				defer rows.Close()
				var resellers []gin.H
				for rows.Next() {
					var resellerID, domain, status, reason string
					var importCount, orderCount int
					rows.Scan(&resellerID, &domain, &status, &reason, &importCount, &orderCount)
					resellers = append(resellers, gin.H{
						"reseller_shop_id": resellerID,
						"domain":           domain,
						"status":           status,
						"reason":           reason,
						"import_count":     importCount,
						"order_count":      orderCount,
					})
				}
				c.JSON(http.StatusOK, gin.H{"resellers": resellers})
			})

			supplier.PUT("/resellers/:resellerId/status", func(c *gin.Context) {
				shopID, _ := c.Get("shop_id")
				var body struct {
					Status string `json:"status" binding:"required"`
					Reason string `json:"reason"`
				}
				if err := c.ShouldBindJSON(&body); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				if body.Status != "active" && body.Status != "paused" && body.Status != "blocked" {
					c.JSON(http.StatusBadRequest, gin.H{"error": "status must be active, paused, or blocked"})
					return
				}

				// Upsert link
				_, err := db.Exec(c.Request.Context(), `
					INSERT INTO supplier_reseller_links (supplier_shop_id, reseller_shop_id, status, reason)
					VALUES ($1, $2, $3, $4)
					ON CONFLICT (supplier_shop_id, reseller_shop_id) DO UPDATE SET
						status = $3, reason = $4, updated_at = NOW()
				`, shopID, c.Param("resellerId"), body.Status, body.Reason)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}

				// If blocked or paused, deactivate all imports from this reseller
				if body.Status == "blocked" || body.Status == "paused" {
					db.Exec(c.Request.Context(), `
						UPDATE reseller_imports SET status = 'paused', last_sync_error = $1
						WHERE reseller_shop_id = $2 AND supplier_listing_id IN (
							SELECT id FROM supplier_listings WHERE supplier_shop_id = $3
						) AND status = 'active'
					`, "Supplier "+body.Status+" your access: "+body.Reason, c.Param("resellerId"), shopID)

					db.Exec(c.Request.Context(), `
						UPDATE product_links SET is_active = FALSE
						WHERE supplier_shop_id = $1 AND reseller_shop_id = $2
					`, shopID, c.Param("resellerId"))

					// Notify reseller
					inappNotifSvc.Create(c.Request.Context(), inappnotif.CreateInput{
						ShopID: c.Param("resellerId"),
						Title: "Access Restricted",
						Message: "A supplier has " + body.Status + " your access. Reason: " + body.Reason,
						Type: "warning",
						Link: strPtr("/imports"),
					})
				} else if body.Status == "active" {
					// Re-activate
					db.Exec(c.Request.Context(), `
						UPDATE reseller_imports SET status = 'active', last_sync_error = NULL
						WHERE reseller_shop_id = $1 AND supplier_listing_id IN (
							SELECT id FROM supplier_listings WHERE supplier_shop_id = $2
						) AND status = 'paused'
					`, c.Param("resellerId"), shopID)

					db.Exec(c.Request.Context(), `
						UPDATE product_links SET is_active = TRUE
						WHERE supplier_shop_id = $1 AND reseller_shop_id = $2
					`, shopID, c.Param("resellerId"))
				}

				auditSvc.Log(c.Request.Context(), shopID.(string), "merchant", shopID.(string), "reseller_status_changed",
					"reseller", c.Param("resellerId"), map[string]string{"status": body.Status, "reason": body.Reason}, "success", "")
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
				// Notify reseller
				var resellerID string
				db.QueryRow(c.Request.Context(), `SELECT reseller_shop_id FROM routed_orders WHERE id = $1`, c.Param("id")).Scan(&resellerID)
				if resellerID != "" {
					inappNotifSvc.Create(c.Request.Context(), inappnotif.CreateInput{ShopID: resellerID, Title: "Order Accepted", Message: "Your order has been accepted by the supplier.", Type: "success", Link: strPtr("/orders/" + c.Param("id"))})
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
				// Notify reseller
				var resellerID string
				db.QueryRow(c.Request.Context(), `SELECT reseller_shop_id FROM routed_orders WHERE id = $1`, c.Param("id")).Scan(&resellerID)
				if resellerID != "" {
					inappNotifSvc.Create(c.Request.Context(), inappnotif.CreateInput{ShopID: resellerID, Title: "Order Rejected", Message: "Your order has been rejected. Reason: " + body.Reason, Type: "error", Link: strPtr("/orders/" + c.Param("id"))})
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
				// Notify reseller about fulfillment
				var resellerID string
				db.QueryRow(c.Request.Context(), `SELECT reseller_shop_id FROM routed_orders WHERE id = $1`, c.Param("id")).Scan(&resellerID)
				if resellerID != "" {
					inappNotifSvc.Create(c.Request.Context(), inappnotif.CreateInput{ShopID: resellerID, Title: "Order Shipped", Message: "Your order has been fulfilled with tracking info.", Type: "success", Link: strPtr("/orders/" + c.Param("id"))})
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

			reseller.PUT("/profile", func(c *gin.Context) {
				shopID, _ := c.Get("shop_id")
				var body map[string]interface{}
				if err := c.ShouldBindJSON(&body); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				if err := shopsSvc.UpdateResellerProfile(c.Request.Context(), shopID.(string), body); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
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

			reseller.GET("/suppliers/:id", func(c *gin.Context) {
				supplierID := c.Param("id")
				// Get shop info
				var shopDomain string
				db.QueryRow(c.Request.Context(), `SELECT COALESCE(shopify_domain,'') FROM shops WHERE id = $1`, supplierID).Scan(&shopDomain)

				// Try to get profile, use defaults if not found
				companyName := shopDomain
				supportEmail := ""
				returnPolicy := ""
				processingDays := 3
				blindFulfillment := false

				profile, err := shopsSvc.GetSupplierProfile(c.Request.Context(), supplierID)
				if err == nil {
					if profile.CompanyName != "" { companyName = profile.CompanyName }
					supportEmail = profile.SupportEmail
					returnPolicy = profile.ReturnPolicyURL
					processingDays = profile.DefaultProcessingDays
					blindFulfillment = profile.BlindFulfillment
				}

				_, listingCount, _ := productsSvc.ListSupplierListings(c.Request.Context(), supplierID, "active", 1, 0)
				c.JSON(http.StatusOK, gin.H{
					"company_name":            companyName,
					"support_email":           supportEmail,
					"return_policy_url":       returnPolicy,
					"default_processing_days": processingDays,
					"blind_fulfillment":       blindFulfillment,
					"listing_count":           listingCount,
				})
			})

			reseller.POST("/imports", func(c *gin.Context) {
				shopID, _ := c.Get("shop_id")
				var input imports.ImportInput
				if err := c.ShouldBindJSON(&input); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}

				// Check if supplier has blocked this reseller
				var supplierID string
				db.QueryRow(c.Request.Context(), `SELECT supplier_shop_id FROM supplier_listings WHERE id = $1`, input.SupplierListingID).Scan(&supplierID)
				if supplierID != "" {
					var linkStatus string
					db.QueryRow(c.Request.Context(), `SELECT status FROM supplier_reseller_links WHERE supplier_shop_id = $1 AND reseller_shop_id = $2`, supplierID, shopID).Scan(&linkStatus)
					if linkStatus == "blocked" || linkStatus == "paused" {
						c.JSON(http.StatusForbidden, gin.H{"error": "This supplier has restricted your access. You cannot import their products."})
						return
					}
				}

				imp, err := importsSvc.Create(c.Request.Context(), shopID.(string), input)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				// Notify supplier about new import
				var supplierShopID string
				db.QueryRow(c.Request.Context(), `SELECT supplier_shop_id FROM supplier_listings WHERE id = $1`, input.SupplierListingID).Scan(&supplierShopID)
				if supplierShopID != "" {
					inappNotifSvc.Create(c.Request.Context(), inappnotif.CreateInput{ShopID: supplierShopID, Title: "Product Imported", Message: "A reseller has imported one of your products.", Type: "info", Link: strPtr("/orders")})
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

			reseller.PUT("/imports/:id/replenish", func(c *gin.Context) {
				shopID, _ := c.Get("shop_id")
				var body struct {
					AutoReplenish      bool `json:"auto_replenish"`
					ReplenishThreshold int  `json:"replenish_threshold"`
					ReplenishQuantity  int  `json:"replenish_quantity"`
				}
				if err := c.ShouldBindJSON(&body); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				_, err := db.Exec(c.Request.Context(), `
					UPDATE reseller_imports SET auto_replenish = $1, replenish_threshold = $2, replenish_quantity = $3
					WHERE id = $4 AND reseller_shop_id = $5
				`, body.AutoReplenish, body.ReplenishThreshold, body.ReplenishQuantity, c.Param("id"), shopID)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})

			reseller.POST("/imports/:id/resync", func(c *gin.Context) {
				shopID, _ := c.Get("shop_id")
				if err := importsSvc.ResyncImport(c.Request.Context(), shopID.(string), c.Param("id")); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})

			reseller.DELETE("/imports/:id", func(c *gin.Context) {
				shopID, _ := c.Get("shop_id")
				if err := importsSvc.DeleteImport(c.Request.Context(), shopID.(string), c.Param("id")); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

		// ===== Disputes (shared) =====
		api.POST("/disputes", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			role, _ := c.Get("shop_role")
			var input disputes.CreateInput
			if err := c.ShouldBindJSON(&input); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			d, err := disputesSvc.Create(c.Request.Context(), shopID.(string), role.(string), input)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			// Notify the other party
			var supplierID, resellerID string
			db.QueryRow(c.Request.Context(), `SELECT supplier_shop_id, reseller_shop_id FROM routed_orders WHERE id = $1`, input.RoutedOrderID).Scan(&supplierID, &resellerID)
			notifyID := supplierID
			if role.(string) == "supplier" { notifyID = resellerID }
			if notifyID != "" {
				inappNotifSvc.Create(c.Request.Context(), inappnotif.CreateInput{ShopID: notifyID, Title: "New Dispute", Message: "A dispute has been opened on an order.", Type: "warning", Link: strPtr("/disputes")})
			}
			c.JSON(http.StatusCreated, d)
		})

		api.GET("/disputes", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			limit := getIntQuery(c, "limit", 20)
			offset := getIntQuery(c, "offset", 0)
			list, total, err := disputesSvc.ListByShop(c.Request.Context(), shopID.(string), limit, offset)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"disputes": list, "total": total})
		})

		api.GET("/disputes/:id", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			d, err := disputesSvc.Get(c.Request.Context(), c.Param("id"), shopID.(string))
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, d)
		})

		api.PUT("/disputes/:id", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			var input disputes.UpdateInput
			if err := c.ShouldBindJSON(&input); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			d, err := disputesSvc.UpdateStatus(c.Request.Context(), c.Param("id"), shopID.(string), input)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, d)
		})

		// ===== In-app Notifications (shared) =====
		api.GET("/notifications", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			limit := getIntQuery(c, "limit", 20)
			offset := getIntQuery(c, "offset", 0)
			notifs, total, err := inappNotifSvc.List(c.Request.Context(), shopID.(string), limit, offset)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"notifications": notifs, "total": total})
		})

		api.POST("/notifications/read/:id", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			if err := inappNotifSvc.MarkRead(c.Request.Context(), c.Param("id"), shopID.(string)); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		api.POST("/notifications/read-all", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			if err := inappNotifSvc.MarkAllRead(c.Request.Context(), shopID.(string)); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		api.GET("/notifications/count", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			count, err := inappNotifSvc.CountUnread(c.Request.Context(), shopID.(string))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"unread_count": count})
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
		api.GET("/billing", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			status, err := billingHandler.GetSvc().GetStatus(c.Request.Context(), shopID.(string))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, status)
		})
		api.GET("/billing/plans", func(c *gin.Context) {
			plans, err := billingHandler.GetSvc().ListPlans(c.Request.Context())
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"plans": plans})
		})
		api.POST("/billing/subscribe", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			var body struct {
				PlanID string `json:"plan_id" binding:"required"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			sub, err := billingHandler.GetSvc().Subscribe(c.Request.Context(), shopID.(string), body.PlanID)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, sub)
		})
		api.POST("/billing/cancel", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			if err := billingHandler.GetSvc().CancelSubscription(c.Request.Context(), shopID.(string)); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		// Supplier Trust/Stats
		api.GET("/supplier-stats/:id", func(c *gin.Context) {
			stats, err := trustSvc.GetStats(c.Request.Context(), c.Param("id"))
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "stats not found"})
				return
			}
			verified := trustSvc.IsVerified(c.Request.Context(), c.Param("id"))
			c.JSON(http.StatusOK, gin.H{"stats": stats, "is_verified": verified})
		})

		// ===== Messaging =====
		api.GET("/conversations", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			convs, err := msgSvc.ListConversations(c.Request.Context(), shopID.(string))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"conversations": convs})
		})

		api.POST("/conversations", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			role, _ := c.Get("shop_role")
			var body struct {
				OtherShopID string `json:"other_shop_id" binding:"required"`
				Subject     string `json:"subject"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			var supplierID, resellerID string
			if role.(string) == "supplier" {
				supplierID = shopID.(string)
				resellerID = body.OtherShopID
			} else {
				supplierID = body.OtherShopID
				resellerID = shopID.(string)
			}
			conv, err := msgSvc.GetOrCreateConversation(c.Request.Context(), supplierID, resellerID, body.Subject)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, conv)
		})

		api.GET("/conversations/:id/messages", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			msgs, err := msgSvc.GetMessages(c.Request.Context(), c.Param("id"), shopID.(string), 100)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"messages": msgs})
		})

		api.POST("/conversations/:id/messages", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			var body struct {
				Content string `json:"content" binding:"required"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			msg, err := msgSvc.SendMessage(c.Request.Context(), c.Param("id"), shopID.(string), body.Content)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			// Notify the other party in the conversation
			var supplierID, resellerID string
			db.QueryRow(c.Request.Context(), `SELECT supplier_shop_id, reseller_shop_id FROM conversations WHERE id = $1`, c.Param("id")).Scan(&supplierID, &resellerID)
			recipientID := supplierID
			if shopID.(string) == supplierID { recipientID = resellerID }
			if recipientID != "" {
				inappNotifSvc.Create(c.Request.Context(), inappnotif.CreateInput{ShopID: recipientID, Title: "New Message", Message: body.Content[:min(len(body.Content), 100)], Type: "info", Link: strPtr("/messages")})
			}
			c.JSON(http.StatusOK, msg)
		})

		// ===== Order Comments =====
		api.GET("/orders/:id/comments", func(c *gin.Context) {
			comments, err := msgSvc.ListOrderComments(c.Request.Context(), c.Param("id"))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"comments": comments})
		})

		api.POST("/orders/:id/comments", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			role, _ := c.Get("shop_role")
			var body struct {
				Content string `json:"content" binding:"required"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			comment, err := msgSvc.AddOrderComment(c.Request.Context(), c.Param("id"), shopID.(string), role.(string), body.Content)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, comment)
		})

		// ===== Announcements =====
		api.GET("/announcements", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			role, _ := c.Get("shop_role")
			anns, err := msgSvc.ListAnnouncements(c.Request.Context(), shopID.(string), role.(string))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"announcements": anns})
		})

		api.POST("/announcements", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			role, _ := c.Get("shop_role")
			if role.(string) != "supplier" {
				c.JSON(http.StatusForbidden, gin.H{"error": "only suppliers can create announcements"})
				return
			}
			var body struct {
				Title    string `json:"title" binding:"required"`
				Content  string `json:"content" binding:"required"`
				IsPinned bool   `json:"is_pinned"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			ann, err := msgSvc.CreateAnnouncement(c.Request.Context(), shopID.(string), body.Title, body.Content, body.IsPinned)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, ann)
		})

		api.POST("/announcements/:id/read", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			msgSvc.MarkAnnouncementRead(c.Request.Context(), c.Param("id"), shopID.(string))
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		api.DELETE("/announcements/:id", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			msgSvc.DeleteAnnouncement(c.Request.Context(), c.Param("id"), shopID.(string))
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		// ===== Reviews =====
		api.GET("/reviews/:supplierID", func(c *gin.Context) {
			reviews, summary, err := advSvc.GetSupplierReviews(c.Request.Context(), c.Param("supplierID"))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"reviews": reviews, "summary": summary})
		})
		api.POST("/reviews", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			var body struct {
				SupplierShopID string  `json:"supplier_shop_id" binding:"required"`
				OrderID        *string `json:"order_id"`
				Rating         int     `json:"rating" binding:"required"`
				Title          string  `json:"title"`
				Comment        string  `json:"comment"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			review, err := advSvc.CreateReview(c.Request.Context(), body.SupplierShopID, shopID.(string), body.OrderID, body.Rating, body.Title, body.Comment)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusCreated, review)
		})

		// ===== Shipping Rules =====
		api.GET("/shipping-rules", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			rules, err := advSvc.ListShippingRules(c.Request.Context(), shopID.(string))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"rules": rules})
		})
		api.POST("/shipping-rules", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			var rule advanced.ShippingRule
			if err := c.ShouldBindJSON(&rule); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			result, err := advSvc.UpsertShippingRule(c.Request.Context(), shopID.(string), rule)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, result)
		})

		// ===== Sample Orders =====
		api.GET("/samples", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			role, _ := c.Get("shop_role")
			samples, err := advSvc.ListSampleOrders(c.Request.Context(), shopID.(string), role.(string))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"samples": samples})
		})
		api.POST("/samples", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			var body struct {
				ListingID string `json:"listing_id" binding:"required"`
				Quantity  int    `json:"quantity"`
				Notes     string `json:"notes"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if body.Quantity <= 0 { body.Quantity = 1 }
			sample, err := advSvc.CreateSampleOrder(c.Request.Context(), shopID.(string), body.ListingID, body.Quantity, body.Notes)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			// Notify supplier about sample request
			var supplierID string
			db.QueryRow(c.Request.Context(), `SELECT supplier_shop_id FROM supplier_listings WHERE id = $1`, body.ListingID).Scan(&supplierID)
			if supplierID != "" {
				inappNotifSvc.Create(c.Request.Context(), inappnotif.CreateInput{ShopID: supplierID, Title: "Sample Request", Message: "A reseller has requested a product sample.", Type: "info", Link: strPtr("/samples")})
			}
			c.JSON(http.StatusCreated, sample)
		})
		api.PUT("/samples/:id", func(c *gin.Context) {
			var body struct {
				Status   string `json:"status"`
				Tracking string `json:"tracking"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if err := advSvc.UpdateSampleOrder(c.Request.Context(), c.Param("id"), body.Status, body.Tracking); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		// ===== Deals =====
		api.GET("/deals", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			role, _ := c.Get("shop_role")
			deals, err := advSvc.ListDeals(c.Request.Context(), shopID.(string), role.(string))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"deals": deals})
		})
		api.POST("/deals", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			var deal advanced.Deal
			if err := c.ShouldBindJSON(&deal); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			result, err := advSvc.CreateDeal(c.Request.Context(), shopID.(string), deal)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusCreated, result)
		})

		// ===== Product Performance =====
		api.GET("/product-performance", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			role, _ := c.Get("shop_role")
			perf, err := advSvc.GetProductPerformance(c.Request.Context(), shopID.(string), role.(string))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"products": perf})
		})

		// ===== Test Order Routing (for development) =====
		api.POST("/test/route-order", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			shopDomain, _ := c.Get("shop_domain")
			role, _ := c.Get("shop_role")
			if role.(string) != "reseller" {
				c.JSON(http.StatusForbidden, gin.H{"error": "only resellers can test"})
				return
			}
			var body struct {
				OrderID int64 `json:"order_id" binding:"required"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			// Fetch order from Shopify REST API
			var encToken string
			db.QueryRow(c.Request.Context(), `SELECT access_token FROM app_installations WHERE shop_id = $1 AND is_active = TRUE`, shopID).Scan(&encToken)
			token, _ := authpkg.Decrypt(encToken, cfg.Security.EncryptionKey)
			domain := ""
			if d, ok := shopDomain.(string); ok { domain = d }

			// Use REST API to get order
			orderURL := fmt.Sprintf("https://%s/admin/api/2024-10/orders/%d.json", domain, body.OrderID)
			req, _ := http.NewRequest("GET", orderURL, nil)
			req.Header.Set("X-Shopify-Access-Token", token)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch order"})
				return
			}
			defer resp.Body.Close()
			var orderResp struct {
				Order map[string]interface{} `json:"order"`
			}
			json.NewDecoder(resp.Body).Decode(&orderResp)
			if orderResp.Order == nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
				return
			}

			err = ordersSvc.RouteOrder(c.Request.Context(), shopID.(string), orderResp.Order)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Order routed! Check supplier's Orders page."})
		})

		// ===== Export =====
		api.GET("/export/orders", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			role, _ := c.Get("shop_role")
			csv, err := advSvc.ExportOrders(c.Request.Context(), shopID.(string), role.(string))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.Header("Content-Type", "text/csv")
			c.Header("Content-Disposition", "attachment; filename=orders.csv")
			c.Data(http.StatusOK, "text/csv", csv)
		})

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

		// ===== Payouts =====
		api.GET("/payouts", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			role, _ := c.Get("shop_role")
			sid := shopID.(string)
			limit := getIntQuery(c, "limit", 20)
			offset := getIntQuery(c, "offset", 0)

			// Count total
			var total int
			shopCol := "reseller_shop_id"
			otherCol := "supplier_shop_id"
			if role.(string) == "supplier" {
				shopCol = "supplier_shop_id"
				otherCol = "reseller_shop_id"
			}
			db.QueryRow(c.Request.Context(), fmt.Sprintf(`SELECT COUNT(*) FROM routed_orders WHERE %s = $1 AND status IN ('pending','accepted','processing','fulfilled')`, shopCol), sid).Scan(&total)

			// Get per-order payouts with product details
			rows, err := db.Query(c.Request.Context(), fmt.Sprintf(`
				SELECT ro.id, ro.reseller_order_number, ro.status, ro.total_wholesale_amount, ro.currency, ro.created_at,
					s.shopify_domain,
					COALESCE(pr.status, 'unpaid') as pay_status,
					COALESCE(pr.platform_fee, 0) as platform_fee,
					COALESCE(pr.supplier_payout, 0) as supplier_payout,
					COALESCE((SELECT string_agg(roi.title, ', ') FROM routed_order_items roi WHERE roi.routed_order_id = ro.id), '') as products,
					COALESCE((SELECT sp.paypal_email FROM supplier_profiles sp WHERE sp.shop_id = ro.supplier_shop_id), '') as supplier_paypal
				FROM routed_orders ro
				JOIN shops s ON s.id = ro.%s
				LEFT JOIN payout_records pr ON pr.routed_order_id = ro.id
				WHERE ro.%s = $1 AND ro.status IN ('pending','accepted','processing','fulfilled')
				ORDER BY ro.created_at DESC
				LIMIT $2 OFFSET $3
			`, otherCol, shopCol), sid, limit, offset)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer rows.Close()

			var payouts []gin.H
			var grandTotal, grandPaid float64
			for rows.Next() {
				var id, orderNum, status, currency, domain, payStatus, products, supplierPaypal string
				var wholesale, fee, payout float64
				var createdAt time.Time
				rows.Scan(&id, &orderNum, &status, &wholesale, &currency, &createdAt, &domain, &payStatus, &fee, &payout, &products, &supplierPaypal)
				payouts = append(payouts, gin.H{
					"id": id, "order_number": orderNum, "status": status,
					"wholesale": wholesale, "currency": currency, "domain": domain,
					"supplier_paypal": supplierPaypal,
					"pay_status": payStatus, "platform_fee": fee, "supplier_payout": payout,
					"products": products, "created_at": createdAt,
				})
				grandTotal += wholesale
				if payStatus == "paid" { grandPaid += payout }
			}
			c.JSON(http.StatusOK, gin.H{
				"payouts": payouts, "total": total,
				"grand_total": grandTotal, "grand_paid": grandPaid,
				"grand_balance": grandTotal - grandPaid,
			})
		})

		// Reseller sends payment
		api.POST("/payouts/send-payment/:orderId", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			role, _ := c.Get("shop_role")
			orderID := c.Param("orderId")

			if role.(string) != "reseller" {
				c.JSON(http.StatusForbidden, gin.H{"error": "only resellers can send payments"})
				return
			}

			var supplierID, resellerID string
			var wholesale float64
			db.QueryRow(c.Request.Context(), `SELECT supplier_shop_id, reseller_shop_id, total_wholesale_amount FROM routed_orders WHERE id = $1`, orderID).Scan(&supplierID, &resellerID, &wholesale)

			// Update to payment_sent or create record
			result, _ := db.Exec(c.Request.Context(), `UPDATE payout_records SET status = 'payment_sent', updated_at = NOW() WHERE routed_order_id = $1 AND status = 'pending'`, orderID)
			if result.RowsAffected() == 0 {
				db.Exec(c.Request.Context(), `
					INSERT INTO payout_records (routed_order_id, supplier_shop_id, reseller_shop_id, wholesale_amount, platform_fee, supplier_payout, status)
					VALUES ($1, $2, $3, $4, 0, $4, 'payment_sent')
				`, orderID, supplierID, resellerID, wholesale)
			}

			// Notify supplier
			inappNotifSvc.Create(c.Request.Context(), inappnotif.CreateInput{
				ShopID: supplierID, Title: "Payment Sent",
				Message: fmt.Sprintf("Reseller sent $%.2f for order. Please confirm receipt.", wholesale),
				Type: "info", Link: strPtr("/payouts"),
			})

			auditSvc.Log(c.Request.Context(), shopID.(string), "merchant", shopID.(string), "payment_sent",
				"payout", orderID, map[string]interface{}{"amount": wholesale}, "success", "")
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		// Supplier confirms payment received
		api.POST("/payouts/confirm-received/:orderId", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			role, _ := c.Get("shop_role")
			orderID := c.Param("orderId")

			if role.(string) != "supplier" {
				c.JSON(http.StatusForbidden, gin.H{"error": "only suppliers can confirm payments"})
				return
			}

			_, err := db.Exec(c.Request.Context(), `UPDATE payout_records SET status = 'paid', updated_at = NOW() WHERE routed_order_id = $1 AND status = 'payment_sent'`, orderID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			// Notify reseller
			var resellerID string
			db.QueryRow(c.Request.Context(), `SELECT reseller_shop_id FROM routed_orders WHERE id = $1`, orderID).Scan(&resellerID)
			if resellerID != "" {
				inappNotifSvc.Create(c.Request.Context(), inappnotif.CreateInput{
					ShopID: resellerID, Title: "Payment Confirmed",
					Message: "Supplier confirmed receipt of your payment.",
					Type: "success", Link: strPtr("/payouts"),
				})
			}

			auditSvc.Log(c.Request.Context(), shopID.(string), "merchant", shopID.(string), "payment_confirmed",
				"payout", orderID, nil, "success", "")
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		// Supplier disputes payment (not received)
		api.POST("/payouts/dispute-payment/:orderId", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			orderID := c.Param("orderId")

			db.Exec(c.Request.Context(), `UPDATE payout_records SET status = 'disputed', updated_at = NOW() WHERE routed_order_id = $1`, orderID)

			var resellerID string
			db.QueryRow(c.Request.Context(), `SELECT reseller_shop_id FROM routed_orders WHERE id = $1`, orderID).Scan(&resellerID)
			if resellerID != "" {
				inappNotifSvc.Create(c.Request.Context(), inappnotif.CreateInput{
					ShopID: resellerID, Title: "Payment Disputed",
					Message: "Supplier says they haven't received your payment. Please check.",
					Type: "warning", Link: strPtr("/payouts"),
				})
			}

			auditSvc.Log(c.Request.Context(), shopID.(string), "merchant", shopID.(string), "payment_disputed",
				"payout", orderID, nil, "failure", "")
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
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
				// SPA fallback: serve index.html with API key injected
				indexPath := filepath.Join(staticDir, "index.html")
				indexBytes, err := os.ReadFile(indexPath)
				if err != nil {
					c.File(indexPath)
					return
				}
				html := strings.Replace(string(indexBytes), `content="" id="shopify-api-key"`, fmt.Sprintf(`content="%s" id="shopify-api-key"`, cfg.Shopify.APIKey), 1)
				c.Header("Content-Type", "text/html; charset=utf-8")
				c.String(http.StatusOK, html)
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

	// Background tasks (replaces worker process)
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				bgCtx := context.Background()
				// Cleanup expired sessions
				db.Exec(bgCtx, `DELETE FROM shop_sessions WHERE expires_at < NOW()`)
				// Cleanup old webhook events
				db.Exec(bgCtx, `DELETE FROM webhook_events WHERE created_at < NOW() - INTERVAL '7 days'`)
				// Update platform stats
				db.Exec(bgCtx, `UPDATE platform_stats SET
					total_products = (SELECT COUNT(*) FROM supplier_listings WHERE status = 'active'),
					total_orders = (SELECT COUNT(*) FROM routed_orders),
					total_suppliers = (SELECT COUNT(*) FROM shops WHERE role = 'supplier' AND status = 'active'),
					total_resellers = (SELECT COUNT(*) FROM shops WHERE role = 'reseller' AND status = 'active'),
					updated_at = NOW() WHERE id = 1`)
				logger.Info().Msg("hourly maintenance completed")
			}
		}
	}()

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

func strPtr(s string) *string { return &s }

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
