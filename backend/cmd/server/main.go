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
	"github.com/droptodrop/droptodrop/internal/jobs"
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

	// Worker for inline job execution (resync etc.)
	jobWorker := jobs.NewWorker(db, redisClient, cfg, logger)

	// Initialize handlers
	authHandler := authpkg.NewHandler(db, cfg.Shopify, cfg.Session, cfg.Security.EncryptionKey, logger, auditSvc)
	healthHandler := health.NewHandler(db, redisClient)
	webhookHandler := webhooks.NewHandler(db, redisClient, shopsSvc, ordersSvc, jobWorker, cfg.Shopify.APISecret, logger, auditSvc)
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

	// ===== Standalone Admin Panel =====
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminPassword == "" {
		adminPassword = "droptodrop2026"
	}

	r.POST("/admin-panel/login", func(c *gin.Context) {
		var body struct{ Password string `json:"password"` }
		c.ShouldBindJSON(&body)
		if body.Password != adminPassword {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "wrong password"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"token": adminPassword})
	})

	adminAPI := r.Group("/admin-panel/api")
	adminAPI.Use(func(c *gin.Context) {
		token := c.GetHeader("X-Admin-Token")
		if token != adminPassword {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	})
	{
		adminAPI.GET("/stats", func(c *gin.Context) {
			ctx := c.Request.Context()
			var totalShops, suppliers, resellers, activeListings, totalImports, totalOrders, pendingOrders, acceptedOrders, fulfilledOrders, rejectedOrders, totalDisputes, openDisputes int
			var totalRevenue, totalPaid, totalPending float64
			db.QueryRow(ctx, `SELECT COUNT(*) FROM shops`).Scan(&totalShops)
			db.QueryRow(ctx, `SELECT COUNT(*) FROM shops WHERE role = 'supplier'`).Scan(&suppliers)
			db.QueryRow(ctx, `SELECT COUNT(*) FROM shops WHERE role = 'reseller'`).Scan(&resellers)
			db.QueryRow(ctx, `SELECT COUNT(*) FROM supplier_listings WHERE status = 'active'`).Scan(&activeListings)
			db.QueryRow(ctx, `SELECT COUNT(*) FROM reseller_imports WHERE status = 'active'`).Scan(&totalImports)
			db.QueryRow(ctx, `SELECT COUNT(*) FROM routed_orders`).Scan(&totalOrders)
			db.QueryRow(ctx, `SELECT COUNT(*) FROM routed_orders WHERE status = 'pending'`).Scan(&pendingOrders)
			db.QueryRow(ctx, `SELECT COUNT(*) FROM routed_orders WHERE status = 'accepted'`).Scan(&acceptedOrders)
			db.QueryRow(ctx, `SELECT COUNT(*) FROM routed_orders WHERE status = 'fulfilled'`).Scan(&fulfilledOrders)
			db.QueryRow(ctx, `SELECT COUNT(*) FROM routed_orders WHERE status = 'rejected'`).Scan(&rejectedOrders)
			db.QueryRow(ctx, `SELECT COALESCE(SUM(total_wholesale_amount), 0) FROM routed_orders`).Scan(&totalRevenue)
			db.QueryRow(ctx, `SELECT COALESCE(SUM(supplier_payout), 0) FROM payout_records WHERE status = 'paid'`).Scan(&totalPaid)
			db.QueryRow(ctx, `SELECT COALESCE(SUM(wholesale_amount), 0) FROM payout_records WHERE status = 'pending'`).Scan(&totalPending)
			db.QueryRow(ctx, `SELECT COUNT(*) FROM disputes`).Scan(&totalDisputes)
			db.QueryRow(ctx, `SELECT COUNT(*) FROM disputes WHERE status = 'open'`).Scan(&openDisputes)
			c.JSON(http.StatusOK, gin.H{
				"total_shops": totalShops, "suppliers": suppliers, "resellers": resellers,
				"active_listings": activeListings, "total_imports": totalImports,
				"total_orders": totalOrders, "pending_orders": pendingOrders, "accepted_orders": acceptedOrders,
				"fulfilled_orders": fulfilledOrders, "rejected_orders": rejectedOrders,
				"total_revenue": totalRevenue, "total_paid": totalPaid, "total_pending": totalPending,
				"total_disputes": totalDisputes, "open_disputes": openDisputes,
			})
		})

		adminAPI.GET("/shops", func(c *gin.Context) {
			ctx := c.Request.Context()
			rows, err := db.Query(ctx, `
				SELECT s.id, s.shopify_domain, COALESCE(s.name,''), s.role, s.status, COALESCE(s.email,''), s.created_at,
					(SELECT COUNT(*) FROM supplier_listings WHERE supplier_shop_id = s.id) as listing_count,
					(SELECT COUNT(*) FROM reseller_imports WHERE reseller_shop_id = s.id) as import_count,
					(SELECT COUNT(*) FROM routed_orders WHERE supplier_shop_id = s.id OR reseller_shop_id = s.id) as order_count,
					COALESCE((SELECT paypal_email FROM supplier_profiles WHERE shop_id = s.id), COALESCE((SELECT paypal_email FROM reseller_profiles WHERE shop_id = s.id), ''))
				FROM shops s ORDER BY s.created_at DESC
			`)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer rows.Close()
			var shops []gin.H
			for rows.Next() {
				var id, domain, name, role, status, email, paypal string
				var createdAt time.Time
				var listingCount, importCount, orderCount int
				rows.Scan(&id, &domain, &name, &role, &status, &email, &createdAt, &listingCount, &importCount, &orderCount, &paypal)
				shops = append(shops, gin.H{
					"id": id, "domain": domain, "name": name, "role": role, "status": status,
					"email": email, "paypal": paypal, "created_at": createdAt,
					"listing_count": listingCount, "import_count": importCount, "order_count": orderCount,
				})
			}
			c.JSON(http.StatusOK, gin.H{"shops": shops})
		})

		adminAPI.GET("/orders", func(c *gin.Context) {
			ctx := c.Request.Context()
			rows, err := db.Query(ctx, `
				SELECT ro.id, COALESCE(ro.reseller_order_number,''), ro.status,
					ro.total_wholesale_amount, ro.currency, COALESCE(ro.customer_shipping_name,''),
					COALESCE(rs.shopify_domain,''), COALESCE(ss.shopify_domain,''),
					COALESCE(pr.status, 'no_payout') as pay_status,
					COALESCE(pr.platform_fee, 0), COALESCE(pr.supplier_payout, 0),
					ro.created_at
				FROM routed_orders ro
				LEFT JOIN shops rs ON rs.id = ro.reseller_shop_id
				LEFT JOIN shops ss ON ss.id = ro.supplier_shop_id
				LEFT JOIN payout_records pr ON pr.routed_order_id = ro.id
				ORDER BY ro.created_at DESC LIMIT 100
			`)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer rows.Close()
			var orders []gin.H
			for rows.Next() {
				var id, orderNum, status, currency, customer, reseller, supplier, payStatus string
				var amount, fee, payout float64
				var createdAt time.Time
				rows.Scan(&id, &orderNum, &status, &amount, &currency, &customer, &reseller, &supplier, &payStatus, &fee, &payout, &createdAt)
				orders = append(orders, gin.H{
					"id": id, "order_number": orderNum, "status": status, "amount": amount,
					"currency": currency, "customer": customer, "reseller": reseller, "supplier": supplier,
					"pay_status": payStatus, "platform_fee": fee, "supplier_payout": payout, "created_at": createdAt,
				})
			}
			c.JSON(http.StatusOK, gin.H{"orders": orders})
		})

		adminAPI.GET("/payouts", func(c *gin.Context) {
			ctx := c.Request.Context()
			rows, err := db.Query(ctx, `
				SELECT pr.id, COALESCE(ro.reseller_order_number,''), pr.status,
					pr.wholesale_amount, pr.platform_fee, pr.supplier_payout,
					COALESCE(rs.shopify_domain,''), COALESCE(ss.shopify_domain,''), pr.created_at
				FROM payout_records pr
				JOIN routed_orders ro ON ro.id = pr.routed_order_id
				LEFT JOIN shops rs ON rs.id = pr.reseller_shop_id
				LEFT JOIN shops ss ON ss.id = pr.supplier_shop_id
				ORDER BY pr.created_at DESC LIMIT 100
			`)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer rows.Close()
			var payouts []gin.H
			for rows.Next() {
				var id, orderNum, status, reseller, supplier string
				var wholesale, fee, payout float64
				var createdAt time.Time
				rows.Scan(&id, &orderNum, &status, &wholesale, &fee, &payout, &reseller, &supplier, &createdAt)
				payouts = append(payouts, gin.H{
					"id": id, "order_number": orderNum, "status": status,
					"wholesale": wholesale, "platform_fee": fee, "supplier_payout": payout,
					"reseller": reseller, "supplier": supplier, "created_at": createdAt,
				})
			}
			c.JSON(http.StatusOK, gin.H{"payouts": payouts})
		})

		adminAPI.GET("/disputes", func(c *gin.Context) {
			ctx := c.Request.Context()
			rows, err := db.Query(ctx, `
				SELECT d.id, COALESCE(ro.reseller_order_number,''), d.dispute_type, d.status,
					d.description, COALESCE(d.resolution,''), d.reporter_role,
					COALESCE(s.shopify_domain,''), d.created_at
				FROM disputes d
				JOIN routed_orders ro ON ro.id = d.routed_order_id
				LEFT JOIN shops s ON s.id = d.reporter_shop_id
				ORDER BY d.created_at DESC LIMIT 100
			`)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer rows.Close()
			var disputes []gin.H
			for rows.Next() {
				var id, orderNum, dtype, status, desc, resolution, role, shop string
				var createdAt time.Time
				rows.Scan(&id, &orderNum, &dtype, &status, &desc, &resolution, &role, &shop, &createdAt)
				disputes = append(disputes, gin.H{
					"id": id, "order_number": orderNum, "type": dtype, "status": status,
					"description": desc, "resolution": resolution, "reporter_role": role,
					"reporter_shop": shop, "created_at": createdAt,
				})
			}
			c.JSON(http.StatusOK, gin.H{"disputes": disputes})
		})

		adminAPI.GET("/activity", func(c *gin.Context) {
			ctx := c.Request.Context()
			rows, err := db.Query(ctx, `
				SELECT al.action, al.resource_type, COALESCE(s.shopify_domain,''), al.outcome, al.created_at
				FROM audit_logs al
				LEFT JOIN shops s ON s.id = al.shop_id
				ORDER BY al.created_at DESC LIMIT 100
			`)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer rows.Close()
			var activity []gin.H
			for rows.Next() {
				var action, resourceType, shop, outcome string
				var createdAt time.Time
				rows.Scan(&action, &resourceType, &shop, &outcome, &createdAt)
				activity = append(activity, gin.H{
					"action": action, "resource_type": resourceType, "shop": shop,
					"outcome": outcome, "created_at": createdAt,
				})
			}
			c.JSON(http.StatusOK, gin.H{"activity": activity})
		})

		// Suspend/activate a shop
		adminAPI.PUT("/shops/:id/status", func(c *gin.Context) {
			var body struct {
				Status string `json:"status" binding:"required"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if body.Status != "active" && body.Status != "suspended" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "status must be 'active' or 'suspended'"})
				return
			}
			result, err := db.Exec(c.Request.Context(), `UPDATE shops SET status = $1, updated_at = NOW() WHERE id = $2`, body.Status, c.Param("id"))
			if err != nil || result.RowsAffected() == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "shop not found"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		// Platform revenue summary
		adminAPI.GET("/revenue", func(c *gin.Context) {
			ctx := c.Request.Context()
			var totalRevenue, totalFees, totalPaid, totalPending, totalDisputed float64
			db.QueryRow(ctx, `SELECT COALESCE(SUM(wholesale_amount), 0), COALESCE(SUM(platform_fee), 0) FROM payout_records`).Scan(&totalRevenue, &totalFees)
			db.QueryRow(ctx, `SELECT COALESCE(SUM(platform_fee), 0) FROM payout_records WHERE status = 'paid'`).Scan(&totalPaid)
			db.QueryRow(ctx, `SELECT COALESCE(SUM(platform_fee), 0) FROM payout_records WHERE status = 'pending'`).Scan(&totalPending)
			db.QueryRow(ctx, `SELECT COALESCE(SUM(platform_fee), 0) FROM payout_records WHERE status = 'disputed'`).Scan(&totalDisputed)

			// Per-shop breakdown
			rows, _ := db.Query(ctx, `
				SELECT COALESCE(s.shopify_domain,''), s.role,
					COALESCE(SUM(pr.wholesale_amount), 0) as total_volume,
					COALESCE(SUM(pr.platform_fee), 0) as total_fees,
					COALESCE(SUM(CASE WHEN pr.status = 'paid' THEN pr.platform_fee ELSE 0 END), 0) as paid_fees,
					COALESCE(SUM(CASE WHEN pr.status = 'pending' THEN pr.platform_fee ELSE 0 END), 0) as pending_fees
				FROM payout_records pr
				JOIN shops s ON s.id = pr.reseller_shop_id
				GROUP BY s.shopify_domain, s.role
				ORDER BY total_fees DESC
			`)
			var shopFees []gin.H
			if rows != nil {
				for rows.Next() {
					var domain, role string
					var volume, fees, paid, pending float64
					rows.Scan(&domain, &role, &volume, &fees, &paid, &pending)
					shopFees = append(shopFees, gin.H{
						"domain": domain, "role": role, "total_volume": volume,
						"total_fees": fees, "paid_fees": paid, "pending_fees": pending,
						"owed": fees - paid,
					})
				}
				rows.Close()
			}

			c.JSON(http.StatusOK, gin.H{
				"total_revenue": totalRevenue, "total_fees": totalFees,
				"paid_fees": totalPaid, "pending_fees": totalPending, "disputed_fees": totalDisputed,
				"shop_breakdown": shopFees,
			})
		})
	}

	// Serve standalone admin panel HTML
	r.GET("/admin-panel", func(c *gin.Context) {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, adminPanelHTML)
	})
	r.GET("/admin-panel/", func(c *gin.Context) {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, adminPanelHTML)
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
			search := c.Query("search")
			query := `
				SELECT s.id, s.shopify_domain, COALESCE(sp.company_name, s.name, s.shopify_domain),
					COALESCE(sp.default_processing_days, 3), COALESCE(sp.is_verified, FALSE),
					(SELECT COUNT(*) FROM supplier_listings sl WHERE sl.supplier_shop_id = s.id AND sl.status = 'active'),
					COALESCE(sp.reliability_score, 0), COALESCE(sp.avg_fulfillment_hours, 0),
					COALESCE(sp.total_orders_fulfilled, 0)
				FROM shops s
				LEFT JOIN supplier_profiles sp ON sp.shop_id = s.id
				WHERE s.role = 'supplier' AND s.status = 'active'`
			args := []interface{}{}
			if search != "" {
				query += ` AND (COALESCE(sp.company_name,'') ILIKE $1 OR s.shopify_domain ILIKE $1 OR s.name ILIKE $1)`
				args = append(args, "%"+search+"%")
			}
			query += ` ORDER BY COALESCE(sp.reliability_score, 0) DESC, (SELECT COUNT(*) FROM supplier_listings sl WHERE sl.supplier_shop_id = s.id AND sl.status = 'active') DESC LIMIT 50`

			rows, err := db.Query(c.Request.Context(), query, args...)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer rows.Close()
			var suppliers []gin.H
			for rows.Next() {
				var id, domain, name string
				var days, fulfilled int
				var verified bool
				var count int
				var score, responseHours float64
				rows.Scan(&id, &domain, &name, &days, &verified, &count, &score, &responseHours, &fulfilled)
				suppliers = append(suppliers, gin.H{
					"id": id, "domain": domain, "name": name,
					"processing_days": days, "verified": verified, "listing_count": count,
					"reliability_score": score, "avg_response_hours": responseHours,
					"total_fulfilled": fulfilled,
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

			// Check if shop has an active installation (access token)
			var hasInstallation bool
			db.QueryRow(c.Request.Context(), `SELECT EXISTS(SELECT 1 FROM app_installations WHERE shop_id = $1 AND is_active = TRUE)`, shopID.(string)).Scan(&hasInstallation)
			if !hasInstallation {
				domain := shop.ShopifyDomain
				if d, ok := shopDomain.(string); ok && d != "" { domain = d }
				authURL := fmt.Sprintf("/auth/install?shop=%s", domain)
				c.JSON(http.StatusOK, gin.H{
					"needs_auth": true,
					"auth_url":   authURL,
					"id":         shop.ID,
					"shopify_domain": shop.ShopifyDomain,
				})
				return
			}

			// Register webhooks and fetch shop info from Shopify
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
							client.RegisterWebhook(bgCtx, topic, cfg.Shopify.AppURL+paths[topic])
						}
						// Fetch and save shop name from Shopify
						shopInfo, err := client.GetShopInfo(bgCtx)
						if err == nil && shopInfo != nil {
							db.Exec(bgCtx, `UPDATE shops SET name = $1, email = $2 WHERE id = $3 AND (name = '' OR name IS NULL)`,
								shopInfo["name"], shopInfo["email"], shopID.(string))
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
				pendingOrders, pendingTotal, _ := ordersSvc.ListRoutedOrders(c.Request.Context(), sid, "supplier", "pending", 100, 0)
				_ = pendingOrders
				dashboard["pending_order_count"] = pendingTotal
				// PayPal + shipping countries check
				var paypal string
				var shippingCountries json.RawMessage
				db.QueryRow(c.Request.Context(), `SELECT COALESCE(paypal_email,''), COALESCE(shipping_countries, '[]'::jsonb) FROM supplier_profiles WHERE shop_id = $1`, sid).Scan(&paypal, &shippingCountries)
				dashboard["paypal_email"] = paypal
				var countries []string
				json.Unmarshal(shippingCountries, &countries)
				dashboard["shipping_countries"] = countries
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
				// PayPal setup check
				var paypal string
				db.QueryRow(c.Request.Context(), `SELECT COALESCE(paypal_email,'') FROM reseller_profiles WHERE shop_id = $1`, sid).Scan(&paypal)
				dashboard["paypal_email"] = paypal
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
				listingID := c.Param("id")
				var input products.UpdateListingInput
				if err := c.ShouldBindJSON(&input); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				if err := productsSvc.UpdateListing(c.Request.Context(), shopID.(string), listingID, input); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}

				// Sync all changes to resellers' Shopify stores in background
				go func() {
					bgCtx := context.Background()
					rows, err := db.Query(bgCtx, `
						SELECT ri.id, ri.reseller_shop_id FROM reseller_imports ri
						WHERE ri.supplier_listing_id = $1 AND ri.status = 'active' AND ri.shopify_product_id IS NOT NULL
					`, listingID)
					if err != nil {
						logger.Error().Err(err).Msg("failed to find reseller imports for sync")
						return
					}
					defer rows.Close()
					for rows.Next() {
						var importID, resellerShopID string
						rows.Scan(&importID, &resellerShopID)
						if err := jobWorker.SyncImportToShopify(bgCtx, importID, resellerShopID); err != nil {
							logger.Error().Err(err).Str("import_id", importID).Msg("failed to sync to reseller")
						} else {
							logger.Info().Str("import_id", importID).Msg("listing synced to reseller Shopify store")
						}
					}
				}()

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
				sid := shopID.(string)
				orderID := c.Param("id")
				if err := ordersSvc.AcceptOrder(c.Request.Context(), orderID, sid); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				// Notify reseller
				var resellerID string
				db.QueryRow(c.Request.Context(), `SELECT reseller_shop_id FROM routed_orders WHERE id = $1`, orderID).Scan(&resellerID)
				if resellerID != "" {
					inappNotifSvc.Create(c.Request.Context(), inappnotif.CreateInput{ShopID: resellerID, Title: "Order Accepted", Message: "Your order has been accepted by the supplier.", Type: "success", Link: strPtr("/orders/" + orderID)})
				}

				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})

			supplier.POST("/orders/:id/reject", func(c *gin.Context) {
				shopID, _ := c.Get("shop_id")
				var body struct {
					Reason string `json:"reason"`
				}
				c.ShouldBindJSON(&body)
				sid := shopID.(string)
				if err := ordersSvc.RejectOrder(c.Request.Context(), c.Param("id"), sid, body.Reason); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				// Notify reseller
				var resellerID string
				db.QueryRow(c.Request.Context(), `SELECT reseller_shop_id FROM routed_orders WHERE id = $1`, c.Param("id")).Scan(&resellerID)
				if resellerID != "" {
					inappNotifSvc.Create(c.Request.Context(), inappnotif.CreateInput{ShopID: resellerID, Title: "Order Rejected", Message: "Your order has been rejected. Reason: " + body.Reason, Type: "error", Link: strPtr("/orders/" + c.Param("id"))})
				}
				// Sync restored inventory to all resellers
				go func() {
					jobWorker.SyncSupplierInventoryToAllResellers(context.Background(), sid)
				}()
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
				sid := shopID.(string)
				// Update supplier fulfillment stats
				db.Exec(c.Request.Context(), `UPDATE supplier_profiles SET total_orders_fulfilled = total_orders_fulfilled + 1 WHERE shop_id = $1`, sid)
				ordersSvc.UpdateReliabilityScore(c.Request.Context(), sid)

				// Notify reseller about fulfillment
				var resellerID string
				db.QueryRow(c.Request.Context(), `SELECT reseller_shop_id FROM routed_orders WHERE id = $1`, c.Param("id")).Scan(&resellerID)
				if resellerID != "" {
					inappNotifSvc.Create(c.Request.Context(), inappnotif.CreateInput{ShopID: resellerID, Title: "Order Shipped", Message: "Your order has been fulfilled with tracking info.", Type: "success", Link: strPtr("/orders/" + c.Param("id"))})
				}

				// Auto-refund sample cost on first sale
				if resellerID != "" {
					// Find unrefunded samples for this reseller from this supplier
					var sampleID string
					var sampleCost float64
					db.QueryRow(c.Request.Context(), `
						SELECT so.id, so.sample_cost FROM sample_orders so
						JOIN supplier_listings sl ON sl.id = so.supplier_listing_id
						WHERE so.reseller_shop_id = $1 AND sl.supplier_shop_id = $2
						AND so.refunded = FALSE AND so.sample_cost > 0 AND so.status IN ('shipped','received')
						LIMIT 1
					`, resellerID, sid).Scan(&sampleID, &sampleCost)
					if sampleID != "" && sampleCost > 0 {
						db.Exec(c.Request.Context(), `UPDATE sample_orders SET refunded = TRUE, refunded_at = NOW() WHERE id = $1`, sampleID)
						inappNotifSvc.Create(c.Request.Context(), inappnotif.CreateInput{
							ShopID: resellerID, Title: "Sample Cost Refunded",
							Message: fmt.Sprintf("Your sample cost of $%.2f has been refunded — your first sale was fulfilled!", sampleCost),
							Type: "success", Link: strPtr("/samples"),
						})
						inappNotifSvc.Create(c.Request.Context(), inappnotif.CreateInput{
							ShopID: sid, Title: "Sample Refund Due",
							Message: fmt.Sprintf("Please refund $%.2f sample cost to the reseller — their first sale was fulfilled.", sampleCost),
							Type: "info", Link: strPtr("/samples"),
						})
						logger.Info().Str("sample_id", sampleID).Float64("cost", sampleCost).Msg("sample cost auto-refund triggered")
					}
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

				// Create product in Shopify immediately (inline, not queued)
				go func() {
					bgCtx := context.Background()
					if err := jobWorker.RunCreateProduct(bgCtx, imp.ID, shopID.(string)); err != nil {
						logger.Error().Err(err).Str("import_id", imp.ID).Msg("inline create_product failed")
					} else {
						logger.Info().Str("import_id", imp.ID).Msg("product created in Shopify via import")
					}
				}()

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
				sid := shopID.(string)
				importID := c.Param("id")

				// Verify import belongs to this reseller
				var exists bool
				db.QueryRow(c.Request.Context(), `SELECT EXISTS(SELECT 1 FROM reseller_imports WHERE id = $1 AND reseller_shop_id = $2)`, importID, sid).Scan(&exists)
				if !exists {
					c.JSON(http.StatusNotFound, gin.H{"error": "import not found"})
					return
				}

				// Delete old Shopify product if it still exists, so we get a clean create
				var oldProductID *int64
				db.QueryRow(c.Request.Context(), `SELECT shopify_product_id FROM reseller_imports WHERE id = $1`, importID).Scan(&oldProductID)
				if oldProductID != nil && *oldProductID > 0 {
					var sDomain, sToken string
					err := db.QueryRow(c.Request.Context(), `SELECT s.shopify_domain, ai.access_token FROM shops s JOIN app_installations ai ON ai.shop_id = s.id AND ai.is_active = TRUE WHERE s.id = $1`, sid).Scan(&sDomain, &sToken)
					if err == nil {
						tok, _ := authpkg.Decrypt(sToken, cfg.Security.EncryptionKey)
						if tok != "" {
							cl := shopify.NewClient(sDomain, tok, logger)
							delQuery := fmt.Sprintf(`mutation { productDelete(input: {id: "gid://shopify/Product/%d"}) { deletedProductId userErrors { field message } } }`, *oldProductID)
							var delResult interface{}
							cl.GraphQL(c.Request.Context(), delQuery, nil, &delResult)
						}
					}
				}

				// Clear old shopify_product_id so handleCreateProduct creates fresh
				db.Exec(c.Request.Context(), `UPDATE reseller_imports SET shopify_product_id = NULL, status = 'pending' WHERE id = $1`, importID)

				// Run the full create_product job inline — same logic used during initial import
				if err := jobWorker.RunCreateProduct(c.Request.Context(), importID, sid); err != nil {
					logger.Error().Err(err).Str("import_id", importID).Msg("resync failed")
					db.Exec(c.Request.Context(), `UPDATE reseller_imports SET status = 'failed', last_sync_error = $2 WHERE id = $1`, importID, err.Error())
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

		// ===== Returns =====
		api.POST("/returns", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			sid := shopID.(string)
			var body struct {
				RoutedOrderID string `json:"routed_order_id" binding:"required"`
				Reason        string `json:"reason" binding:"required"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			// Get order details
			var supplierID, customerName string
			err := db.QueryRow(c.Request.Context(), `
				SELECT supplier_shop_id, customer_shipping_name FROM routed_orders
				WHERE id = $1 AND reseller_shop_id = $2 AND status = 'fulfilled'
			`, body.RoutedOrderID, sid).Scan(&supplierID, &customerName)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "order not found or not fulfilled"})
				return
			}
			var returnID string
			err = db.QueryRow(c.Request.Context(), `
				INSERT INTO return_requests (routed_order_id, reseller_shop_id, supplier_shop_id, reason, customer_name)
				VALUES ($1, $2, $3, $4, $5) RETURNING id
			`, body.RoutedOrderID, sid, supplierID, body.Reason, customerName).Scan(&returnID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			// Notify supplier
			inappNotifSvc.Create(c.Request.Context(), inappnotif.CreateInput{
				ShopID: supplierID, Title: "Return Request",
				Message: "A customer wants to return an order. Please upload a return label.",
				Type: "warning", Link: strPtr("/returns"),
			})
			c.JSON(http.StatusCreated, gin.H{"id": returnID, "status": "requested"})
		})

		api.GET("/returns", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			role, _ := c.Get("shop_role")
			sid := shopID.(string)
			shopCol := "reseller_shop_id"
			if role.(string) == "supplier" { shopCol = "supplier_shop_id" }
			rows, err := db.Query(c.Request.Context(), fmt.Sprintf(`
				SELECT rr.id, rr.routed_order_id, COALESCE(ro.reseller_order_number,''), rr.status, rr.reason,
					COALESCE(rr.customer_name,''), COALESCE(rr.return_label_url,''), COALESCE(rr.supplier_notes,''),
					COALESCE(rs.shopify_domain,''), COALESCE(ss.shopify_domain,''), rr.created_at
				FROM return_requests rr
				JOIN routed_orders ro ON ro.id = rr.routed_order_id
				LEFT JOIN shops rs ON rs.id = rr.reseller_shop_id
				LEFT JOIN shops ss ON ss.id = rr.supplier_shop_id
				WHERE rr.%s = $1 ORDER BY rr.created_at DESC LIMIT 50
			`, shopCol), sid)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer rows.Close()
			var returns []gin.H
			for rows.Next() {
				var id, orderID, orderNum, status, reason, customer, labelURL, notes, reseller, supplier string
				var createdAt time.Time
				rows.Scan(&id, &orderID, &orderNum, &status, &reason, &customer, &labelURL, &notes, &reseller, &supplier, &createdAt)
				returns = append(returns, gin.H{
					"id": id, "order_id": orderID, "order_number": orderNum, "status": status,
					"reason": reason, "customer_name": customer, "return_label_url": labelURL,
					"supplier_notes": notes, "reseller": reseller, "supplier": supplier, "created_at": createdAt,
				})
			}
			c.JSON(http.StatusOK, gin.H{"returns": returns})
		})

		// Supplier uploads return label
		api.PUT("/returns/:id/label", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			var body struct {
				LabelURL string `json:"label_url" binding:"required"`
				Notes    string `json:"notes"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			result, err := db.Exec(c.Request.Context(), `
				UPDATE return_requests SET return_label_url = $1, supplier_notes = $2, status = 'label_uploaded', updated_at = NOW()
				WHERE id = $3 AND supplier_shop_id = $4 AND status = 'requested'
			`, body.LabelURL, body.Notes, c.Param("id"), shopID)
			if err != nil || result.RowsAffected() == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "return not found or already processed"})
				return
			}
			// Notify reseller
			var resellerID string
			db.QueryRow(c.Request.Context(), `SELECT reseller_shop_id FROM return_requests WHERE id = $1`, c.Param("id")).Scan(&resellerID)
			if resellerID != "" {
				inappNotifSvc.Create(c.Request.Context(), inappnotif.CreateInput{
					ShopID: resellerID, Title: "Return Label Ready",
					Message: "Supplier uploaded a return shipping label. Share it with your customer.",
					Type: "success", Link: strPtr("/returns"),
				})
			}
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		// Update return status
		api.PUT("/returns/:id/status", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			var body struct {
				Status string `json:"status" binding:"required"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			db.Exec(c.Request.Context(), `
				UPDATE return_requests SET status = $1, updated_at = NOW()
				WHERE id = $2 AND (supplier_shop_id = $3 OR reseller_shop_id = $3)
			`, body.Status, c.Param("id"), shopID)
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
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

		// Nav badge counts
		api.GET("/nav-counts", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			role, _ := c.Get("shop_role")
			sid := shopID.(string)
			r := role.(string)
			ctx := c.Request.Context()

			counts := gin.H{}

			// Orders: pending/accepted (need attention)
			var orderCol string
			if r == "supplier" {
				orderCol = "supplier_shop_id"
			} else {
				orderCol = "reseller_shop_id"
			}
			var orderCount int
			db.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM routed_orders WHERE %s = $1 AND status IN ('pending','accepted')`, orderCol), sid).Scan(&orderCount)
			counts["orders"] = orderCount

			// Unread messages
			var msgCount int
			db.QueryRow(ctx, `
				SELECT COUNT(*) FROM messages m
				JOIN conversations cv ON cv.id = m.conversation_id
				WHERE (cv.supplier_shop_id = $1 OR cv.reseller_shop_id = $1)
				AND m.sender_shop_id != $1 AND m.is_read = FALSE
			`, sid).Scan(&msgCount)
			counts["messages"] = msgCount

			// Unread notifications
			var notifCount int
			db.QueryRow(ctx, `SELECT COUNT(*) FROM notifications WHERE shop_id = $1 AND is_read = FALSE`, sid).Scan(&notifCount)
			counts["notifications"] = notifCount

			// Payouts needing action — match what the Payouts page shows
			var payoutCount int
			if r == "supplier" {
				// Supplier: payments sent but not yet confirmed
				db.QueryRow(ctx, `
					SELECT COUNT(*) FROM routed_orders ro
					JOIN payout_records pr ON pr.routed_order_id = ro.id
					WHERE ro.supplier_shop_id = $1 AND pr.status = 'payment_sent'
				`, sid).Scan(&payoutCount)
			} else {
				// Reseller: orders with unpaid/disputed payouts
				db.QueryRow(ctx, `
					SELECT COUNT(*) FROM routed_orders ro
					JOIN payout_records pr ON pr.routed_order_id = ro.id
					WHERE ro.reseller_shop_id = $1 AND pr.status IN ('pending','disputed')
					AND ro.status IN ('pending','accepted','processing','fulfilled')
				`, sid).Scan(&payoutCount)
			}
			counts["payouts"] = payoutCount

			// Open disputes
			var disputeCount int
			db.QueryRow(ctx, `SELECT COUNT(*) FROM disputes WHERE (complainant_shop_id = $1 OR respondent_shop_id = $1) AND status = 'open'`, sid).Scan(&disputeCount)
			counts["disputes"] = disputeCount

			c.JSON(http.StatusOK, counts)
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
		api.PUT("/shipping-rules/:id", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			var rule advanced.ShippingRule
			if err := c.ShouldBindJSON(&rule); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			_, err := db.Exec(c.Request.Context(), `
				UPDATE shipping_rules SET country_code=$1, shipping_rate=$2, free_shipping_threshold=$3,
					estimated_days_min=$4, estimated_days_max=$5, updated_at=NOW()
				WHERE id=$6 AND shop_id=$7
			`, rule.CountryCode, rule.ShippingRate, rule.FreeShippingThreshold,
				rule.EstDaysMin, rule.EstDaysMax, c.Param("id"), shopID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})
		api.DELETE("/shipping-rules/:id", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			_, err := db.Exec(c.Request.Context(), `DELETE FROM shipping_rules WHERE id=$1 AND shop_id=$2`, c.Param("id"), shopID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
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
			sid := shopID.(string)
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

			// Limit: one sample per product per reseller
			var existingCount int
			db.QueryRow(c.Request.Context(), `SELECT COUNT(*) FROM sample_orders WHERE reseller_shop_id = $1 AND supplier_listing_id = $2`, sid, body.ListingID).Scan(&existingCount)
			if existingCount > 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "You have already requested a sample for this product"})
				return
			}

			// Get wholesale price for sample cost
			var sampleCost float64
			db.QueryRow(c.Request.Context(), `SELECT COALESCE(wholesale_price, 0) FROM supplier_listing_variants WHERE listing_id = $1 LIMIT 1`, body.ListingID).Scan(&sampleCost)

			sample, err := advSvc.CreateSampleOrder(c.Request.Context(), sid, body.ListingID, body.Quantity, body.Notes)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			// Set sample cost
			db.Exec(c.Request.Context(), `UPDATE sample_orders SET sample_cost = $1 WHERE id = $2`, sampleCost, sample.ID)

			// Notify supplier about sample request
			var supplierID string
			db.QueryRow(c.Request.Context(), `SELECT supplier_shop_id FROM supplier_listings WHERE id = $1`, body.ListingID).Scan(&supplierID)
			if supplierID != "" {
				inappNotifSvc.Create(c.Request.Context(), inappnotif.CreateInput{
					ShopID: supplierID, Title: "Sample Request",
					Message: fmt.Sprintf("A reseller has requested a product sample. Sample cost: $%.2f (refunded after first sale).", sampleCost),
					Type: "info", Link: strPtr("/samples"),
				})
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

			sid := shopID.(string)
			err = ordersSvc.RouteOrder(c.Request.Context(), sid, orderResp.Order)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			// Run post-routing tasks inline
			go func() {
				bgCtx := context.Background()
				rows, qErr := db.Query(bgCtx, `
					SELECT id, supplier_shop_id, total_wholesale_amount FROM routed_orders
					WHERE reseller_shop_id = $1 AND status = 'pending' AND supplier_notified_at IS NULL
					ORDER BY created_at DESC LIMIT 5
				`, sid)
				if qErr == nil {
					defer rows.Close()
					for rows.Next() {
						var routedID, supplierID string
						var wholesale float64
						rows.Scan(&routedID, &supplierID, &wholesale)
						jobWorker.RunSupplierNotification(bgCtx, routedID, supplierID)
						jobWorker.RunChargeOrder(bgCtx, routedID, sid, supplierID, wholesale)
						// Sync inventory to ALL resellers
						jobWorker.SyncSupplierInventoryToAllResellers(bgCtx, supplierID)
					}
				}
			}()

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

		// ===== Admin Panel =====
		api.GET("/admin/dashboard", func(c *gin.Context) {
			shopDomain, _ := c.Get("shop_domain")
			adminDomain := os.Getenv("ADMIN_SHOP_DOMAIN")
			if adminDomain == "" {
				adminDomain = "gmurys-vt.myshopify.com" // default admin
			}
			domain := ""
			if d, ok := shopDomain.(string); ok { domain = d }
			// Also check by shop ID
			shopID, _ := c.Get("shop_id")
			var actualDomain string
			db.QueryRow(c.Request.Context(), `SELECT shopify_domain FROM shops WHERE id = $1`, shopID).Scan(&actualDomain)
			if domain != adminDomain && actualDomain != adminDomain {
				c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
				return
			}

			ctx := c.Request.Context()
			result := gin.H{}

			// Stats
			var totalShops, suppliers, resellers, activeListings, totalImports, totalOrders, pendingOrders, fulfilledOrders int
			var totalRevenue, totalPayouts float64
			db.QueryRow(ctx, `SELECT COUNT(*) FROM shops`).Scan(&totalShops)
			db.QueryRow(ctx, `SELECT COUNT(*) FROM shops WHERE role = 'supplier'`).Scan(&suppliers)
			db.QueryRow(ctx, `SELECT COUNT(*) FROM shops WHERE role = 'reseller'`).Scan(&resellers)
			db.QueryRow(ctx, `SELECT COUNT(*) FROM supplier_listings WHERE status = 'active'`).Scan(&activeListings)
			db.QueryRow(ctx, `SELECT COUNT(*) FROM reseller_imports`).Scan(&totalImports)
			db.QueryRow(ctx, `SELECT COUNT(*) FROM routed_orders`).Scan(&totalOrders)
			db.QueryRow(ctx, `SELECT COUNT(*) FROM routed_orders WHERE status = 'pending'`).Scan(&pendingOrders)
			db.QueryRow(ctx, `SELECT COUNT(*) FROM routed_orders WHERE status = 'fulfilled'`).Scan(&fulfilledOrders)
			db.QueryRow(ctx, `SELECT COALESCE(SUM(total_wholesale_amount), 0) FROM routed_orders`).Scan(&totalRevenue)
			db.QueryRow(ctx, `SELECT COALESCE(SUM(supplier_payout), 0) FROM payout_records WHERE status = 'paid'`).Scan(&totalPayouts)

			result["stats"] = gin.H{
				"total_shops": totalShops, "suppliers": suppliers, "resellers": resellers,
				"active_listings": activeListings, "total_imports": totalImports,
				"total_orders": totalOrders, "pending_orders": pendingOrders, "fulfilled_orders": fulfilledOrders,
				"total_revenue": totalRevenue, "total_payouts": totalPayouts,
			}

			// All shops
			shopRows, _ := db.Query(ctx, `SELECT id, shopify_domain, COALESCE(name,''), role, status, created_at FROM shops ORDER BY created_at DESC`)
			var shops []gin.H
			if shopRows != nil {
				for shopRows.Next() {
					var id, domain, name, role, status string
					var createdAt time.Time
					shopRows.Scan(&id, &domain, &name, &role, &status, &createdAt)
					shops = append(shops, gin.H{"id": id, "shopify_domain": domain, "name": name, "role": role, "status": status, "created_at": createdAt})
				}
				shopRows.Close()
			}
			result["shops"] = shops

			// Recent orders
			orderRows, _ := db.Query(ctx, `
				SELECT ro.id, COALESCE(ro.reseller_order_number,''), ro.status, ro.total_wholesale_amount, ro.currency,
					COALESCE(rs.shopify_domain,'') as reseller, COALESCE(ss.shopify_domain,'') as supplier, ro.created_at
				FROM routed_orders ro
				LEFT JOIN shops rs ON rs.id = ro.reseller_shop_id
				LEFT JOIN shops ss ON ss.id = ro.supplier_shop_id
				ORDER BY ro.created_at DESC LIMIT 50
			`)
			var orders []gin.H
			if orderRows != nil {
				for orderRows.Next() {
					var id, orderNum, status, currency, resellerDomain, supplierDomain string
					var amount float64
					var createdAt time.Time
					orderRows.Scan(&id, &orderNum, &status, &amount, &currency, &resellerDomain, &supplierDomain, &createdAt)
					orders = append(orders, gin.H{"id": id, "order_number": orderNum, "status": status, "amount": amount, "currency": currency, "reseller": resellerDomain, "supplier": supplierDomain, "created_at": createdAt})
				}
				orderRows.Close()
			}
			result["recent_orders"] = orders

			// Recent activity
			actRows, _ := db.Query(ctx, `
				SELECT al.action, al.resource_type, COALESCE(s.shopify_domain,''), al.created_at
				FROM audit_logs al
				LEFT JOIN shops s ON s.id = al.shop_id
				ORDER BY al.created_at DESC LIMIT 50
			`)
			var activity []gin.H
			if actRows != nil {
				for actRows.Next() {
					var action, resourceType, shopDomain string
					var createdAt time.Time
					actRows.Scan(&action, &resourceType, &shopDomain, &createdAt)
					activity = append(activity, gin.H{"action": action, "resource_type": resourceType, "shop_domain": shopDomain, "created_at": createdAt})
				}
				actRows.Close()
			}
			result["recent_activity"] = activity

			c.JSON(http.StatusOK, result)
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
			var grandTotal, grandPaid, grandFees float64
			for rows.Next() {
				var id, orderNum, status, currency, domain, payStatus, products, supplierPaypal string
				var wholesale, fee, payout float64
				var createdAt time.Time
				rows.Scan(&id, &orderNum, &status, &wholesale, &currency, &createdAt, &domain, &payStatus, &fee, &payout, &products, &supplierPaypal)
				// If payout not yet calculated, use wholesale
				if payout == 0 { payout = wholesale }
				payouts = append(payouts, gin.H{
					"id": id, "order_number": orderNum, "status": status,
					"wholesale": wholesale, "currency": currency, "domain": domain,
					"supplier_paypal": supplierPaypal,
					"pay_status": payStatus, "platform_fee": fee, "supplier_payout": payout,
					"products": products, "created_at": createdAt,
				})
				grandTotal += payout
				grandFees += fee
				if payStatus == "paid" { grandPaid += payout }
			}
			// Check if this shop has PayPal set up
			var myPaypal string
			if role.(string) == "supplier" {
				db.QueryRow(c.Request.Context(), `SELECT COALESCE(paypal_email,'') FROM supplier_profiles WHERE shop_id = $1`, sid).Scan(&myPaypal)
			} else {
				db.QueryRow(c.Request.Context(), `SELECT COALESCE(paypal_email,'') FROM reseller_profiles WHERE shop_id = $1`, sid).Scan(&myPaypal)
			}

			c.JSON(http.StatusOK, gin.H{
				"payouts": payouts, "total": total,
				"grand_total": grandTotal, "grand_paid": grandPaid,
				"grand_balance": grandTotal - grandPaid,
				"grand_fees": grandFees,
				"has_paypal": myPaypal != "",
			})
		})

		// Reseller sends payment
		api.POST("/payouts/send-payment/:orderId", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			role, _ := c.Get("shop_role")
			orderID := c.Param("orderId")
			sid := shopID.(string)

			if role.(string) != "reseller" {
				c.JSON(http.StatusForbidden, gin.H{"error": "only resellers can send payments"})
				return
			}

			var supplierID, resellerID string
			var wholesale float64
			err := db.QueryRow(c.Request.Context(), `SELECT supplier_shop_id, reseller_shop_id, total_wholesale_amount FROM routed_orders WHERE id = $1 AND reseller_shop_id = $2`, orderID, sid).Scan(&supplierID, &resellerID, &wholesale)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
				return
			}

			// Calculate fee from billing plan — charged on RETAIL price
			feePercent := 2.0
			db.QueryRow(c.Request.Context(), `
				SELECT bp.app_fee_percent FROM shop_subscriptions ss
				JOIN billing_plans bp ON bp.id = ss.plan_id
				WHERE ss.shop_id = $1 AND ss.status = 'active'
			`, sid).Scan(&feePercent)
			// Get retail price from markup
			var markupType string
			var markupValue float64
			db.QueryRow(c.Request.Context(), `
				SELECT COALESCE(ri.markup_type,'percentage'), COALESCE(ri.markup_value, 30)
				FROM routed_orders ro
				JOIN reseller_imports ri ON ri.reseller_shop_id = ro.reseller_shop_id
				WHERE ro.id = $1 LIMIT 1
			`, orderID).Scan(&markupType, &markupValue)
			var retailPrice float64
			if markupType == "fixed" {
				retailPrice = wholesale + markupValue
			} else {
				retailPrice = wholesale * (1 + markupValue/100)
			}
			if retailPrice < wholesale { retailPrice = wholesale }
			platformFee := retailPrice * feePercent / 100
			supplierPayout := wholesale - platformFee
			if supplierPayout < 0 { supplierPayout = 0 }

			// Update to payment_sent or create record
			result, _ := db.Exec(c.Request.Context(), `UPDATE payout_records SET status = 'payment_sent', updated_at = NOW() WHERE routed_order_id = $1 AND status IN ('pending', 'disputed')`, orderID)
			if result.RowsAffected() == 0 {
				db.Exec(c.Request.Context(), `
					INSERT INTO payout_records (routed_order_id, supplier_shop_id, reseller_shop_id, wholesale_amount, platform_fee, supplier_payout, status)
					VALUES ($1, $2, $3, $4, $5, $6, 'payment_sent')
				`, orderID, supplierID, resellerID, wholesale, platformFee, supplierPayout)
			}

			// Notify supplier
			inappNotifSvc.Create(c.Request.Context(), inappnotif.CreateInput{
				ShopID: supplierID, Title: "Payment Sent",
				Message: fmt.Sprintf("Reseller sent $%.2f for order. Please confirm receipt.", supplierPayout),
				Type: "info", Link: strPtr("/payouts"),
			})

			auditSvc.Log(c.Request.Context(), sid, "merchant", sid, "payment_sent",
				"payout", orderID, map[string]interface{}{"wholesale": wholesale, "fee": platformFee, "supplier_payout": supplierPayout}, "success", "")
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		// Supplier confirms payment received
		api.POST("/payouts/confirm-received/:orderId", func(c *gin.Context) {
			shopID, _ := c.Get("shop_id")
			role, _ := c.Get("shop_role")
			orderID := c.Param("orderId")
			sid := shopID.(string)

			if role.(string) != "supplier" {
				c.JSON(http.StatusForbidden, gin.H{"error": "only suppliers can confirm payments"})
				return
			}

			_, err := db.Exec(c.Request.Context(), `UPDATE payout_records SET status = 'paid', updated_at = NOW() WHERE routed_order_id = $1 AND supplier_shop_id = $2 AND status = 'payment_sent'`, orderID, sid)
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

			sid := shopID.(string)
			db.Exec(c.Request.Context(), `UPDATE payout_records SET status = 'disputed', updated_at = NOW() WHERE routed_order_id = $1 AND supplier_shop_id = $2`, orderID, sid)

			var resellerID string
			db.QueryRow(c.Request.Context(), `SELECT reseller_shop_id FROM routed_orders WHERE id = $1 AND supplier_shop_id = $2`, orderID, sid).Scan(&resellerID)
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
				!strings.HasPrefix(path, "/webhooks/") && !strings.HasPrefix(path, "/health") &&
				!strings.HasPrefix(path, "/admin-panel") {
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
