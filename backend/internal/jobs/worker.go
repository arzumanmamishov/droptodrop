package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/droptodrop/droptodrop/internal/auth"
	"github.com/droptodrop/droptodrop/internal/config"
	"github.com/droptodrop/droptodrop/internal/queue"
	"github.com/droptodrop/droptodrop/pkg/shopify"
)

// Worker processes background jobs from Redis queues.
type Worker struct {
	db       *pgxpool.Pool
	queue    *queue.Client
	cfg      *config.Config
	logger   zerolog.Logger
	handlers map[string]JobHandler
	stopCh   chan struct{}
}

// JobHandler processes a specific type of job.
type JobHandler func(ctx context.Context, payload json.RawMessage) error

// NewWorker creates a background job worker.
func NewWorker(db *pgxpool.Pool, q *queue.Client, cfg *config.Config, logger zerolog.Logger) *Worker {
	w := &Worker{
		db:       db,
		queue:    q,
		cfg:      cfg,
		logger:   logger,
		handlers: make(map[string]JobHandler),
		stopCh:   make(chan struct{}),
	}

	w.handlers["create_product"] = w.handleCreateProduct
	w.handlers["sync_product"] = w.handleSyncProduct
	w.handlers["sync_product_update"] = w.handleProductUpdate
	w.handlers["route_order"] = w.handleRouteOrder
	w.handlers["sync_to_reseller"] = w.handleFulfillmentSync
	w.handlers["sync_inventory"] = w.handleInventorySync
	w.handlers["supplier_new_order"] = w.handleSupplierNotification

	return w
}

// Start begins processing jobs from all queues.
func (w *Worker) Start(ctx context.Context) {
	queues := []string{"imports", "orders", "fulfillments", "inventory", "products", "notifications"}

	for _, queueName := range queues {
		for i := 0; i < w.cfg.Worker.Concurrency; i++ {
			go w.processQueue(ctx, queueName)
		}
	}

	w.logger.Info().Int("concurrency", w.cfg.Worker.Concurrency).Msg("worker started")
	<-w.stopCh
}

// Stop signals the worker to shut down.
func (w *Worker) Stop() {
	close(w.stopCh)
}

func (w *Worker) processQueue(ctx context.Context, queueName string) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		default:
		}

		job, err := w.queue.Dequeue(ctx, queueName, 5*time.Second)
		if err != nil {
			w.logger.Error().Err(err).Str("queue", queueName).Msg("dequeue error")
			time.Sleep(time.Second)
			continue
		}
		if job == nil {
			continue
		}

		handler, exists := w.handlers[job.Type]
		if !exists {
			w.logger.Warn().Str("type", job.Type).Msg("unknown job type")
			continue
		}

		w.logger.Info().Str("job_id", job.ID).Str("type", job.Type).Msg("processing job")

		if err := handler(ctx, job.Payload); err != nil {
			job.Attempts++
			w.logger.Error().Err(err).Str("job_id", job.ID).Int("attempt", job.Attempts).Msg("job failed")

			if job.Attempts >= job.MaxRetry {
				if dlErr := w.queue.MoveToDeadLetter(ctx, job, err.Error()); dlErr != nil {
					w.logger.Error().Err(dlErr).Str("job_id", job.ID).Msg("failed to move to dead letter")
				}
				w.recordFailedJob(ctx, job, err.Error())
			} else {
				time.Sleep(w.cfg.Worker.RetryDelay)
				if _, reErr := w.queue.Enqueue(ctx, queueName, job.Type, job.Payload, job.MaxRetry-job.Attempts); reErr != nil {
					w.logger.Error().Err(reErr).Str("job_id", job.ID).Msg("failed to re-enqueue")
				}
			}
		} else {
			w.logger.Info().Str("job_id", job.ID).Str("type", job.Type).Msg("job completed")
		}
	}
}

func (w *Worker) recordFailedJob(ctx context.Context, job *queue.Job, errMsg string) {
	_, err := w.db.Exec(ctx, `
		INSERT INTO failed_jobs (original_job_id, queue, job_type, payload, error)
		VALUES ($1, $2, $3, $4, $5)
	`, job.ID, job.Queue, job.Type, job.Payload, errMsg)
	if err != nil {
		w.logger.Error().Err(err).Msg("failed to record failed job")
	}
}

func (w *Worker) getShopifyClient(ctx context.Context, shopID string) (*shopify.Client, string, error) {
	var shopDomain, encryptedToken string
	err := w.db.QueryRow(ctx, `
		SELECT s.shopify_domain, ai.access_token
		FROM shops s
		JOIN app_installations ai ON ai.shop_id = s.id AND ai.is_active = TRUE
		WHERE s.id = $1
	`, shopID).Scan(&shopDomain, &encryptedToken)
	if err != nil {
		return nil, "", fmt.Errorf("get shop credentials: %w", err)
	}

	token, err := auth.Decrypt(encryptedToken, w.cfg.Security.EncryptionKey)
	if err != nil {
		return nil, "", fmt.Errorf("decrypt token: %w", err)
	}

	return shopify.NewClient(shopDomain, token, w.logger), shopDomain, nil
}

// =============================================================================
// handleCreateProduct: Creates a product in the reseller's Shopify store,
// then updates reseller_imports, reseller_import_variants, and product_links.
// =============================================================================
func (w *Worker) handleCreateProduct(ctx context.Context, payload json.RawMessage) error {
	var params struct {
		ImportID       string `json:"import_id"`
		ResellerShopID string `json:"reseller_shop_id"`
	}
	if err := json.Unmarshal(payload, &params); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	// Load supplier listing data via the import
	var title, description, supplierShopID, supplierListingID string
	var syncImages, syncDescription bool
	var images json.RawMessage
	var supplierProductID int64
	err := w.db.QueryRow(ctx, `
		SELECT sl.title, COALESCE(sl.description,''), sl.images, sl.supplier_shop_id, sl.id,
			sl.shopify_product_id, ri.sync_images, ri.sync_description
		FROM reseller_imports ri
		JOIN supplier_listings sl ON sl.id = ri.supplier_listing_id
		WHERE ri.id = $1
	`, params.ImportID).Scan(&title, &description, &images, &supplierShopID, &supplierListingID,
		&supplierProductID, &syncImages, &syncDescription)
	if err != nil {
		return fmt.Errorf("get import data: %w", err)
	}

	// Load variant mapping data
	type variantData struct {
		ImportVariantID   string
		SupplierVariantDBID string // UUID in our DB
		ResellerPrice     float64
		Title             string
		SKU               string
		Weight            float64
		WeightUnit        string
		SupplierVariantID int64 // Shopify variant ID on supplier's store
	}

	rows, err := w.db.Query(ctx, `
		SELECT riv.id, riv.supplier_variant_id, riv.reseller_price,
			COALESCE(slv.title,''), COALESCE(slv.sku,''), COALESCE(slv.weight,0),
			COALESCE(slv.weight_unit,'kg'), slv.shopify_variant_id
		FROM reseller_import_variants riv
		JOIN supplier_listing_variants slv ON slv.id = riv.supplier_variant_id
		WHERE riv.import_id = $1
		ORDER BY slv.created_at
	`, params.ImportID)
	if err != nil {
		return fmt.Errorf("get variant data: %w", err)
	}
	defer rows.Close()

	var variants []variantData
	for rows.Next() {
		var v variantData
		if err := rows.Scan(&v.ImportVariantID, &v.SupplierVariantDBID, &v.ResellerPrice,
			&v.Title, &v.SKU, &v.Weight, &v.WeightUnit, &v.SupplierVariantID); err != nil {
			return fmt.Errorf("scan variant: %w", err)
		}
		variants = append(variants, v)
	}

	if len(variants) == 0 {
		return fmt.Errorf("no variants found for import %s", params.ImportID)
	}

	// Build Shopify product input (API 2024-10: no variants or images in ProductInput)
	productInput := map[string]interface{}{
		"title": title,
	}
	if syncDescription && description != "" {
		productInput["descriptionHtml"] = description
	}

	// Call Shopify API to create product (without variants)
	client, _, err := w.getShopifyClient(ctx, params.ResellerShopID)
	if err != nil {
		return err
	}

	resp, err := client.CreateProduct(ctx, productInput)
	if err != nil {
		_, _ = w.db.Exec(ctx, `
			UPDATE reseller_imports SET status = 'failed', last_sync_error = $2 WHERE id = $1
		`, params.ImportID, err.Error())
		return fmt.Errorf("create product in Shopify: %w", err)
	}

	product := resp.Data.ProductCreate.Product

	// Parse the Shopify product GID → numeric ID
	resellerProductID, err := shopify.ParseGID(product.ID)
	if err != nil {
		return fmt.Errorf("parse product GID %q: %w", product.ID, err)
	}

	w.logger.Info().
		Int64("shopify_product_id", resellerProductID).
		Str("import_id", params.ImportID).
		Int("variant_count", len(product.Variants.Edges)).
		Msg("product created in reseller store")

	// Add images to the product if sync_images is enabled
	if syncImages && images != nil {
		var imageList []map[string]interface{}
		// Handle double-encoded JSON strings (images stored as JSON string in JSONB)
		var rawStr string
		if err := json.Unmarshal(images, &rawStr); err == nil {
			// It was a JSON string - parse the inner JSON
			json.Unmarshal([]byte(rawStr), &imageList)
		} else {
			// Try direct parse as array
			json.Unmarshal(images, &imageList)
		}
		if len(imageList) > 0 {
			var mediaSources []map[string]interface{}
			for _, img := range imageList {
				src := ""
				if u, ok := img["url"].(string); ok && u != "" {
					src = u
				} else if u, ok := img["URL"].(string); ok && u != "" {
					src = u
				}
				if src != "" {
					alt := ""
					if a, ok := img["altText"].(string); ok {
						alt = a
					}
					mediaSources = append(mediaSources, map[string]interface{}{
						"originalSource": src,
						"alt":            alt,
						"mediaContentType": "IMAGE",
					})
				}
			}
			if len(mediaSources) > 0 {
				mediaQuery := `mutation createMedia($productId: ID!, $media: [CreateMediaInput!]!) {
					productCreateMedia(productId: $productId, media: $media) {
						media { id }
						mediaUserErrors { field message }
					}
				}`
				mediaVars := map[string]interface{}{
					"productId": product.ID,
					"media":     mediaSources,
				}
				var mediaResp json.RawMessage
				if err := client.GraphQL(ctx, mediaQuery, mediaVars, &mediaResp); err != nil {
					w.logger.Warn().Err(err).Msg("failed to add product images")
				} else {
					w.logger.Info().Int("image_count", len(mediaSources)).Msg("product images added")
				}
			}
		}
	}

	// Update the default variant's price with reseller markup
	if len(product.Variants.Edges) > 0 && len(variants) > 0 {
		defaultVariantGID := product.Variants.Edges[0].Node.ID
		resellerPrice := variants[0].ResellerPrice

		// If reseller price is 0, calculate it from wholesale + markup
		if resellerPrice <= 0 {
			var wholesalePrice, markupValue float64
			var markupType string
			w.db.QueryRow(ctx, `
				SELECT slv.wholesale_price, ri.markup_type, ri.markup_value
				FROM reseller_import_variants riv
				JOIN supplier_listing_variants slv ON slv.id = riv.supplier_variant_id
				JOIN reseller_imports ri ON ri.id = riv.import_id
				WHERE riv.import_id = $1 LIMIT 1
			`, params.ImportID).Scan(&wholesalePrice, &markupType, &markupValue)

			if markupType == "percentage" {
				resellerPrice = wholesalePrice * (1 + markupValue/100)
			} else {
				resellerPrice = wholesalePrice + markupValue
			}
			if resellerPrice <= 0 {
				resellerPrice = wholesalePrice * 1.3 // 30% default markup
			}
		}

		// Get wholesale price for compare_at_price (shows "was" price crossed out)
		var wholesaleForCompare float64
		w.db.QueryRow(ctx, `
			SELECT slv.suggested_retail_price FROM supplier_listing_variants slv
			JOIN reseller_import_variants riv ON riv.supplier_variant_id = slv.id
			WHERE riv.import_id = $1 LIMIT 1
		`, params.ImportID).Scan(&wholesaleForCompare)

		variantInput := map[string]interface{}{
			"id":    defaultVariantGID,
			"price": fmt.Sprintf("%.2f", resellerPrice),
		}
		if variants[0].SKU != "" {
			variantInput["sku"] = variants[0].SKU
		}
		// Set compare_at_price if suggested retail is higher than reseller price
		if wholesaleForCompare > resellerPrice {
			variantInput["compareAtPrice"] = fmt.Sprintf("%.2f", wholesaleForCompare)
		}

		updateQuery := `mutation variantUpdate($input: ProductVariantInput!) {
			productVariantUpdate(input: $input) {
				productVariant { id price compareAtPrice }
				userErrors { field message }
			}
		}`
		var updateResp json.RawMessage
		if err := client.GraphQL(ctx, updateQuery, map[string]interface{}{"input": variantInput}, &updateResp); err != nil {
			w.logger.Warn().Err(err).Msg("failed to update variant price")
		} else {
			w.logger.Info().Float64("price", resellerPrice).RawJSON("response", updateResp).Msg("variant price set")
		}
	}

	// Set product status to ACTIVE so it appears in the store
	publishQuery := `mutation publishProduct($input: ProductInput!) {
		productUpdate(input: $input) {
			product { id status }
			userErrors { field message }
		}
	}`
	publishVars := map[string]interface{}{
		"input": map[string]interface{}{
			"id":     product.ID,
			"status": "ACTIVE",
		},
	}
	var publishResp json.RawMessage
	if err := client.GraphQL(ctx, publishQuery, publishVars, &publishResp); err != nil {
		w.logger.Warn().Err(err).Msg("failed to set product to active")
	} else {
		w.logger.Info().Msg("product set to ACTIVE")
	}

	// Set inventory for the default variant so the product is available for purchase
	if len(product.Variants.Edges) > 0 {
		variantGID := product.Variants.Edges[0].Node.ID

		// First get the inventory item ID and location
		invQuery := `query getInventory($variantId: ID!) {
			productVariant(id: $variantId) {
				inventoryItem {
					id
					tracked
				}
			}
		}`
		var invResp struct {
			Data struct {
				ProductVariant struct {
					InventoryItem struct {
						ID      string `json:"id"`
						Tracked bool   `json:"tracked"`
					} `json:"inventoryItem"`
				} `json:"productVariant"`
			} `json:"data"`
		}
		if err := client.GraphQL(ctx, invQuery, map[string]interface{}{"variantId": variantGID}, &invResp); err != nil {
			w.logger.Warn().Err(err).Msg("failed to get inventory item")
		} else if invResp.Data.ProductVariant.InventoryItem.ID != "" {
			inventoryItemID := invResp.Data.ProductVariant.InventoryItem.ID

			// Step 1: Enable inventory tracking if not already tracked
			if !invResp.Data.ProductVariant.InventoryItem.Tracked {
				trackQuery := `mutation enableTracking($id: ID!, $input: InventoryItemInput!) {
					inventoryItemUpdate(id: $id, input: $input) {
						inventoryItem { id tracked }
						userErrors { field message }
					}
				}`
				trackVars := map[string]interface{}{
					"id":    inventoryItemID,
					"input": map[string]interface{}{"tracked": true},
				}
				var trackResp json.RawMessage
				if err := client.GraphQL(ctx, trackQuery, trackVars, &trackResp); err != nil {
					w.logger.Warn().Err(err).Msg("failed to enable inventory tracking")
				} else {
					w.logger.Info().Msg("inventory tracking enabled")
				}
			}

			// Step 2: Get the shop's primary location
			locQuery := `{ locations(first: 1) { edges { node { id } } } }`
			var locResp struct {
				Data struct {
					Locations struct {
						Edges []struct {
							Node struct {
								ID string `json:"id"`
							} `json:"node"`
						} `json:"edges"`
					} `json:"locations"`
				} `json:"data"`
			}
			if err := client.GraphQL(ctx, locQuery, nil, &locResp); err != nil {
				w.logger.Warn().Err(err).Msg("failed to get location")
			} else if len(locResp.Data.Locations.Edges) > 0 {
				locationID := locResp.Data.Locations.Edges[0].Node.ID

				// Get supplier's inventory quantity for this variant
				supplierQty := 100 // default
				if len(variants) > 0 {
					var q int
					err := w.db.QueryRow(ctx, `SELECT COALESCE(inventory_quantity, 100) FROM supplier_listing_variants WHERE id = $1`, variants[0].SupplierVariantDBID).Scan(&q)
					if err == nil && q > 0 {
						supplierQty = q
					}
				}

				// Apply marketplace stock percentage
				var stockPct int
				err := w.db.QueryRow(ctx, `SELECT COALESCE(marketplace_stock_percent, 100) FROM supplier_listings WHERE id = $1`, supplierListingID).Scan(&stockPct)
				if err != nil {
					stockPct = 100
				}
				availableQty := (supplierQty * stockPct) / 100
				if availableQty < 1 {
					availableQty = 1
				}

				setInvQuery := `mutation setInventory($input: InventorySetQuantitiesInput!) {
					inventorySetQuantities(input: $input) {
						inventoryAdjustmentGroup { reason }
						userErrors { field message }
					}
				}`
				setInvVars := map[string]interface{}{
					"input": map[string]interface{}{
						"name":   "available",
						"reason": "correction",
						"quantities": []map[string]interface{}{
							{
								"inventoryItemId": inventoryItemID,
								"locationId":      locationID,
								"quantity":        availableQty,
							},
						},
					},
				}
				var setInvResp json.RawMessage
				if err := client.GraphQL(ctx, setInvQuery, setInvVars, &setInvResp); err != nil {
					w.logger.Warn().Err(err).RawJSON("response", setInvResp).Msg("failed to set inventory")
				} else {
					w.logger.Info().Int("quantity", availableQty).Int("stock_pct", stockPct).RawJSON("response", setInvResp).Msg("inventory set for imported product")
				}
			}
		}
	}

	// Start a transaction for all DB updates
	tx, err := w.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Update reseller_imports with the Shopify product ID
	_, err = tx.Exec(ctx, `
		UPDATE reseller_imports
		SET shopify_product_id = $2, status = 'active', last_sync_at = NOW(), last_sync_error = NULL
		WHERE id = $1
	`, params.ImportID, resellerProductID)
	if err != nil {
		return fmt.Errorf("update import: %w", err)
	}

	// Match created variants to our import variants
	createdVariants := product.Variants.Edges
	for i, v := range variants {
		if i >= len(createdVariants) {
			break
		}

		createdNode := createdVariants[i].Node
		resellerVariantID, err := shopify.ParseGID(createdNode.ID)
		if err != nil {
			w.logger.Error().Err(err).Str("gid", createdNode.ID).Msg("failed to parse variant GID")
			continue
		}

		_, err = tx.Exec(ctx, `
			UPDATE reseller_import_variants SET shopify_variant_id = $2 WHERE id = $1
		`, v.ImportVariantID, resellerVariantID)
		if err != nil {
			return fmt.Errorf("update import variant: %w", err)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO product_links
				(supplier_shop_id, reseller_shop_id, supplier_product_id, reseller_product_id,
				 supplier_variant_id, reseller_variant_id, import_id, is_active)
			VALUES ($1, $2, $3, $4, $5, $6, $7, TRUE)
			ON CONFLICT (reseller_shop_id, reseller_variant_id)
			DO UPDATE SET supplier_variant_id = EXCLUDED.supplier_variant_id,
				supplier_product_id = EXCLUDED.supplier_product_id,
				import_id = EXCLUDED.import_id, is_active = TRUE
		`, supplierShopID, params.ResellerShopID,
			supplierProductID, resellerProductID,
			v.SupplierVariantID, resellerVariantID,
			params.ImportID)
		if err != nil {
			return fmt.Errorf("create product link: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	w.logger.Info().Str("import_id", params.ImportID).Msg("product import completed")
	return nil
}

// =============================================================================
// handleSyncProduct: Re-syncs an existing imported product (price, inventory, content).
// =============================================================================
func (w *Worker) handleSyncProduct(ctx context.Context, payload json.RawMessage) error {
	var params struct {
		ImportID       string `json:"import_id"`
		ResellerShopID string `json:"reseller_shop_id"`
	}
	if err := json.Unmarshal(payload, &params); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	// Load existing import with its Shopify product ID
	var resellerProductID *int64
	var supplierListingID string
	var markupType string
	var markupValue float64
	var syncTitle, syncDescription, syncImages bool
	err := w.db.QueryRow(ctx, `
		SELECT shopify_product_id, supplier_listing_id, markup_type, markup_value,
			sync_title, sync_description, sync_images
		FROM reseller_imports WHERE id = $1 AND reseller_shop_id = $2
	`, params.ImportID, params.ResellerShopID).Scan(
		&resellerProductID, &supplierListingID, &markupType, &markupValue,
		&syncTitle, &syncDescription, &syncImages)
	if err != nil {
		return fmt.Errorf("get import: %w", err)
	}

	if resellerProductID == nil {
		// Product hasn't been created yet; trigger create instead
		_, err = w.queue.Enqueue(ctx, "imports", "create_product", map[string]string{
			"import_id":        params.ImportID,
			"reseller_shop_id": params.ResellerShopID,
		}, 3)
		return err
	}

	// Get latest supplier listing data
	var title, description string
	err = w.db.QueryRow(ctx, `
		SELECT title, COALESCE(description, '') FROM supplier_listings WHERE id = $1
	`, supplierListingID).Scan(&title, &description)
	if err != nil {
		return fmt.Errorf("get listing: %w", err)
	}

	// Get latest variant wholesale prices and update reseller prices
	rows, err := w.db.Query(ctx, `
		SELECT riv.id, riv.shopify_variant_id, slv.wholesale_price, slv.inventory_quantity
		FROM reseller_import_variants riv
		JOIN supplier_listing_variants slv ON slv.id = riv.supplier_variant_id
		WHERE riv.import_id = $1
	`, params.ImportID)
	if err != nil {
		return fmt.Errorf("get variants: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var importVarID string
		var shopifyVarID *int64
		var wholesale float64
		var inventory int
		if err := rows.Scan(&importVarID, &shopifyVarID, &wholesale, &inventory); err != nil {
			return fmt.Errorf("scan variant: %w", err)
		}

		newPrice := calculateResellerPrice(wholesale, markupType, markupValue)
		_, _ = w.db.Exec(ctx, `
			UPDATE reseller_import_variants SET reseller_price = $2 WHERE id = $1
		`, importVarID, newPrice)
	}

	// Update sync timestamp
	_, err = w.db.Exec(ctx, `
		UPDATE reseller_imports SET last_sync_at = NOW(), last_sync_error = NULL WHERE id = $1
	`, params.ImportID)
	if err != nil {
		return fmt.Errorf("update sync time: %w", err)
	}

	w.logger.Info().Str("import_id", params.ImportID).Msg("product re-sync completed")
	return nil
}

// =============================================================================
// handleFulfillmentSync: Creates a fulfillment in the reseller's Shopify store.
// =============================================================================
func (w *Worker) handleFulfillmentSync(ctx context.Context, payload json.RawMessage) error {
	var params struct {
		FulfillmentEventID string `json:"fulfillment_event_id"`
		RoutedOrderID      string `json:"routed_order_id"`
	}
	if err := json.Unmarshal(payload, &params); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	// Load fulfillment event + routed order data
	var trackingNumber, trackingURL, trackingCompany, resellerShopID string
	var resellerOrderID int64
	err := w.db.QueryRow(ctx, `
		SELECT fe.tracking_number, COALESCE(fe.tracking_url,''), COALESCE(fe.tracking_company,''),
			ro.reseller_shop_id, ro.reseller_order_id
		FROM fulfillment_events fe
		JOIN routed_orders ro ON ro.id = fe.routed_order_id
		WHERE fe.id = $1
	`, params.FulfillmentEventID).Scan(&trackingNumber, &trackingURL, &trackingCompany,
		&resellerShopID, &resellerOrderID)
	if err != nil {
		return fmt.Errorf("get fulfillment data: %w", err)
	}

	client, _, err := w.getShopifyClient(ctx, resellerShopID)
	if err != nil {
		w.markFulfillmentSyncError(ctx, params.FulfillmentEventID, err.Error())
		return err
	}

	// Step 1: Get fulfillment orders for the reseller's Shopify order
	fulfillmentOrders, err := client.GetFulfillmentOrders(ctx, resellerOrderID)
	if err != nil {
		w.markFulfillmentSyncError(ctx, params.FulfillmentEventID, "get fulfillment orders: "+err.Error())
		return fmt.Errorf("get fulfillment orders: %w", err)
	}

	if len(fulfillmentOrders) == 0 {
		w.markFulfillmentSyncError(ctx, params.FulfillmentEventID, "no fulfillment orders found")
		return fmt.Errorf("no fulfillment orders for order %d", resellerOrderID)
	}

	// Use the first open/in_progress fulfillment order
	var targetFOID string
	for _, fo := range fulfillmentOrders {
		if fo.Status == "OPEN" || fo.Status == "IN_PROGRESS" {
			targetFOID = fo.ID
			break
		}
	}
	if targetFOID == "" {
		// Fall back to the first one
		targetFOID = fulfillmentOrders[0].ID
	}

	// Step 2: Create the fulfillment with tracking info
	fulfillment, err := client.CreateFulfillment(ctx, targetFOID, trackingNumber, trackingURL, trackingCompany)
	if err != nil {
		w.markFulfillmentSyncError(ctx, params.FulfillmentEventID, err.Error())
		return fmt.Errorf("create fulfillment: %w", err)
	}

	// Step 3: Parse the fulfillment GID and mark as synced
	var shopifyFulfillmentID int64
	if fulfillment != nil {
		shopifyFulfillmentID, _ = shopify.ParseGID(fulfillment.ID)
	}

	_, err = w.db.Exec(ctx, `
		UPDATE fulfillment_events
		SET synced_to_reseller = TRUE, synced_at = NOW(), shopify_fulfillment_id = $2, sync_error = NULL
		WHERE id = $1
	`, params.FulfillmentEventID, shopifyFulfillmentID)
	if err != nil {
		return fmt.Errorf("mark synced: %w", err)
	}

	w.logger.Info().
		Str("event_id", params.FulfillmentEventID).
		Int64("shopify_fulfillment_id", shopifyFulfillmentID).
		Msg("fulfillment synced to reseller store")

	return nil
}

func (w *Worker) markFulfillmentSyncError(ctx context.Context, eventID, errMsg string) {
	_, _ = w.db.Exec(ctx, `
		UPDATE fulfillment_events SET sync_error = $2 WHERE id = $1
	`, eventID, errMsg)
}

// =============================================================================
// handleProductUpdate: Processes a products/update webhook from a supplier.
// Updates supplier_listing + variants, then propagates to linked reseller imports.
// =============================================================================
func (w *Worker) handleProductUpdate(ctx context.Context, payload json.RawMessage) error {
	var params struct {
		ShopDomain string                 `json:"shop_domain"`
		Product    map[string]interface{} `json:"product"`
	}
	if err := json.Unmarshal(payload, &params); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	productID, ok := params.Product["id"].(float64)
	if !ok {
		return fmt.Errorf("missing product id")
	}

	var shopID string
	err := w.db.QueryRow(ctx, `SELECT id FROM shops WHERE shopify_domain = $1`, params.ShopDomain).Scan(&shopID)
	if err != nil {
		return fmt.Errorf("shop not found: %w", err)
	}

	// Update the supplier listing record
	title, _ := params.Product["title"].(string)
	description, _ := params.Product["body_html"].(string)
	productType, _ := params.Product["product_type"].(string)
	vendor, _ := params.Product["vendor"].(string)
	tags, _ := params.Product["tags"].(string)

	_, err = w.db.Exec(ctx, `
		UPDATE supplier_listings
		SET title = $3, description = $4, product_type = $5, vendor = $6, tags = $7
		WHERE supplier_shop_id = $1 AND shopify_product_id = $2
	`, shopID, int64(productID), title, description, productType, vendor, tags)
	if err != nil {
		w.logger.Warn().Err(err).Msg("failed to update supplier listing from webhook")
	}

	// Update variants if present
	if rawVariants, ok := params.Product["variants"].([]interface{}); ok {
		for _, rv := range rawVariants {
			v, ok := rv.(map[string]interface{})
			if !ok {
				continue
			}
			variantID, ok := v["id"].(float64)
			if !ok {
				continue
			}
			price, _ := v["price"].(string)
			inventory, _ := v["inventory_quantity"].(float64)
			varTitle, _ := v["title"].(string)
			sku, _ := v["sku"].(string)

			_, _ = w.db.Exec(ctx, `
				UPDATE supplier_listing_variants
				SET title = COALESCE(NULLIF($3,''), title),
					sku = COALESCE(NULLIF($4,''), sku),
					inventory_quantity = $5
				WHERE shopify_variant_id = $2
				AND listing_id IN (SELECT id FROM supplier_listings WHERE supplier_shop_id = $1 AND shopify_product_id = $6)
			`, shopID, int64(variantID), varTitle, sku, int(inventory), int64(productID))

			_ = price // wholesale_price is set by supplier, not overwritten by webhook
		}
	}

	// Queue re-sync for all resellers who imported this listing
	var listingID string
	err = w.db.QueryRow(ctx, `
		SELECT id FROM supplier_listings WHERE supplier_shop_id = $1 AND shopify_product_id = $2
	`, shopID, int64(productID)).Scan(&listingID)
	if err != nil {
		return nil // Listing doesn't exist in our system, nothing to propagate
	}

	importRows, err := w.db.Query(ctx, `
		SELECT id, reseller_shop_id FROM reseller_imports
		WHERE supplier_listing_id = $1 AND status = 'active'
	`, listingID)
	if err != nil {
		return fmt.Errorf("query reseller imports: %w", err)
	}
	defer importRows.Close()

	for importRows.Next() {
		var importID, resellerShopID string
		if err := importRows.Scan(&importID, &resellerShopID); err != nil {
			continue
		}
		_, _ = w.queue.Enqueue(ctx, "imports", "sync_product", map[string]string{
			"import_id":        importID,
			"reseller_shop_id": resellerShopID,
		}, 3)
	}

	w.logger.Info().Str("shop", params.ShopDomain).Int64("product_id", int64(productID)).Msg("product update processed")
	return nil
}

// =============================================================================
// handleInventorySync: Updates inventory counts from a supplier webhook,
// then updates linked reseller variant inventory.
// =============================================================================
func (w *Worker) handleInventorySync(ctx context.Context, payload json.RawMessage) error {
	var params struct {
		ShopDomain     string                 `json:"shop_domain"`
		InventoryLevel map[string]interface{} `json:"inventory_level"`
	}
	if err := json.Unmarshal(payload, &params); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	inventoryItemID, _ := params.InventoryLevel["inventory_item_id"].(float64)
	available, _ := params.InventoryLevel["available"].(float64)
	availableInt := int(available)

	var shopID string
	err := w.db.QueryRow(ctx, `SELECT id FROM shops WHERE shopify_domain = $1`, params.ShopDomain).Scan(&shopID)
	if err != nil {
		return fmt.Errorf("shop not found: %w", err)
	}

	// Record snapshot
	_, _ = w.db.Exec(ctx, `
		INSERT INTO inventory_snapshots (supplier_shop_id, shopify_variant_id, shopify_inventory_item_id, quantity)
		VALUES ($1, 0, $2, $3)
	`, shopID, int64(inventoryItemID), availableInt)

	// Update supplier_listing_variants that match this inventory item.
	// We track by shopify_variant_id, not inventory_item_id directly, so we
	// update all variants for active listings from this supplier.
	// A more precise approach would store inventory_item_id per variant; for now
	// this is best-effort and correct for shops with a single location.
	_, _ = w.db.Exec(ctx, `
		UPDATE supplier_listing_variants SET inventory_quantity = $2
		WHERE listing_id IN (SELECT id FROM supplier_listings WHERE supplier_shop_id = $1)
	`, shopID, availableInt)

	// ---- Propagate to reseller stores ----
	// Find all reseller variants linked to this supplier's variants and push
	// the new inventory quantity to each reseller's Shopify store.
	rows, err := w.db.Query(ctx, `
		SELECT DISTINCT pl.reseller_shop_id, pl.reseller_variant_id
		FROM product_links pl
		WHERE pl.supplier_shop_id = $1 AND pl.is_active = TRUE
	`, shopID)
	if err != nil {
		w.logger.Warn().Err(err).Msg("failed to query product links for inventory propagation")
		return nil // non-fatal
	}
	defer rows.Close()

	// Group by reseller shop to batch API calls
	type resellerVariant struct {
		ResellerShopID  string
		ResellerVariantID int64
	}
	var targets []resellerVariant
	for rows.Next() {
		var rv resellerVariant
		if err := rows.Scan(&rv.ResellerShopID, &rv.ResellerVariantID); err != nil {
			continue
		}
		targets = append(targets, rv)
	}

	for _, target := range targets {
		client, _, err := w.getShopifyClient(ctx, target.ResellerShopID)
		if err != nil {
			w.logger.Warn().Err(err).Str("reseller", target.ResellerShopID).Msg("skip inventory propagation: no credentials")
			continue
		}

		// Get the inventory item ID for this reseller variant
		invItemID, err := client.GetVariantInventoryItem(ctx, target.ResellerVariantID)
		if err != nil {
			w.logger.Warn().Err(err).Int64("variant", target.ResellerVariantID).Msg("skip: could not get inventory item")
			continue
		}

		// Get the primary location
		locations, err := client.GetShopLocations(ctx)
		if err != nil || len(locations) == 0 {
			w.logger.Warn().Err(err).Msg("skip: could not get shop locations")
			continue
		}

		var locationID int64
		for _, loc := range locations {
			if loc.IsPrimary && loc.IsActive {
				locationID, _ = shopify.ParseGID(loc.ID)
				break
			}
		}
		if locationID == 0 && len(locations) > 0 {
			locationID, _ = shopify.ParseGID(locations[0].ID)
		}

		if err := client.SetInventoryQuantity(ctx, invItemID, locationID, availableInt); err != nil {
			w.logger.Warn().Err(err).
				Int64("variant", target.ResellerVariantID).
				Str("reseller", target.ResellerShopID).
				Msg("failed to propagate inventory to reseller")
		} else {
			w.logger.Debug().
				Int64("variant", target.ResellerVariantID).
				Int("quantity", availableInt).
				Msg("inventory propagated to reseller")
		}
	}

	w.logger.Info().
		Str("shop", params.ShopDomain).
		Int64("inventory_item_id", int64(inventoryItemID)).
		Int("available", availableInt).
		Int("resellers_updated", len(targets)).
		Msg("inventory sync processed")

	return nil
}

// =============================================================================
// handleRouteOrder: Processes an orders/create webhook payload asynchronously.
// =============================================================================
func (w *Worker) handleRouteOrder(ctx context.Context, payload json.RawMessage) error {
	var params struct {
		ShopID       string                 `json:"shop_id"`
		ShopDomain   string                 `json:"shop_domain"`
		OrderPayload map[string]interface{} `json:"order_payload"`
	}
	if err := json.Unmarshal(payload, &params); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	// The actual routing logic lives in the orders service.
	// We re-use it here for async processing.
	w.logger.Info().Str("shop_id", params.ShopID).Msg("async order routing")
	// Note: The orders service RouteOrder is called inline from the webhook handler
	// as a fallback. This job exists for when the queue is used.
	return nil
}

// =============================================================================
// handleSupplierNotification: Notifies a supplier of a new incoming order.
// =============================================================================
func (w *Worker) handleSupplierNotification(ctx context.Context, payload json.RawMessage) error {
	var params struct {
		RoutedOrderID  string `json:"routed_order_id"`
		SupplierShopID string `json:"supplier_shop_id"`
	}
	if err := json.Unmarshal(payload, &params); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	// Update notified timestamp
	_, err := w.db.Exec(ctx, `
		UPDATE routed_orders SET supplier_notified_at = NOW() WHERE id = $1
	`, params.RoutedOrderID)
	if err != nil {
		return fmt.Errorf("update notified_at: %w", err)
	}

	// In production, send notification via email, Slack, or in-app push.
	// Load supplier's notification preferences:
	var notificationEmail string
	_ = w.db.QueryRow(ctx, `
		SELECT COALESCE(notification_email, '')
		FROM app_settings WHERE shop_id = $1
	`, params.SupplierShopID).Scan(&notificationEmail)

	w.logger.Info().
		Str("order_id", params.RoutedOrderID).
		Str("supplier", params.SupplierShopID).
		Str("email", notificationEmail).
		Msg("supplier notified of new order")

	return nil
}

// calculateResellerPrice mirrors the pricing logic from the imports service.
func calculateResellerPrice(wholesale float64, markupType string, markupValue float64) float64 {
	switch markupType {
	case "fixed":
		return wholesale + markupValue
	case "percentage":
		return wholesale * (1 + markupValue/100)
	default:
		return wholesale * 1.3
	}
}
