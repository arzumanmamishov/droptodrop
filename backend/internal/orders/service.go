package orders

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/droptodrop/droptodrop/internal/audit"
	"github.com/droptodrop/droptodrop/internal/queue"
	"github.com/droptodrop/droptodrop/pkg/idempotency"
)

// RoutedOrder represents an order routed from reseller to supplier.
type RoutedOrder struct {
	ID                     string            `json:"id"`
	ResellerShopID         string            `json:"reseller_shop_id"`
	SupplierShopID         string            `json:"supplier_shop_id"`
	ResellerOrderID        int64             `json:"reseller_order_id"`
	ResellerOrderNumber    string            `json:"reseller_order_number"`
	Status                 string            `json:"status"`
	CustomerShippingName   string            `json:"customer_shipping_name"`
	CustomerShippingAddr   json.RawMessage   `json:"customer_shipping_address"`
	CustomerEmail          string            `json:"customer_email"`
	CustomerPhone          string            `json:"customer_phone"`
	TotalWholesaleAmount   float64           `json:"total_wholesale_amount"`
	Currency               string            `json:"currency"`
	Notes                  string            `json:"notes"`
	ResellerShopName       string            `json:"reseller_shop_name,omitempty"`
	SupplierShopName       string            `json:"supplier_shop_name,omitempty"`
	Items                  []RoutedOrderItem `json:"items,omitempty"`
	CreatedAt              time.Time         `json:"created_at"`
	UpdatedAt              time.Time         `json:"updated_at"`
}

// RoutedOrderItem represents a line item in a routed order.
type RoutedOrderItem struct {
	ID                 string  `json:"id"`
	RoutedOrderID      string  `json:"routed_order_id"`
	ResellerLineItemID int64   `json:"reseller_line_item_id"`
	SupplierVariantID  int64   `json:"supplier_variant_id"`
	ResellerVariantID  int64   `json:"reseller_variant_id"`
	Title              string  `json:"title"`
	SKU                string  `json:"sku"`
	Quantity           int     `json:"quantity"`
	WholesaleUnitPrice float64 `json:"wholesale_unit_price"`
	FulfillmentStatus  string  `json:"fulfillment_status"`
	FulfilledQuantity  int     `json:"fulfilled_quantity"`
	ImageURL           string  `json:"image_url,omitempty"`
}

// Service handles order routing operations.
type Service struct {
	db     *pgxpool.Pool
	queue  *queue.Client
	logger zerolog.Logger
	audit  *audit.Service
}

// NewService creates an order service.
func NewService(db *pgxpool.Pool, q *queue.Client, logger zerolog.Logger, auditSvc *audit.Service) *Service {
	return &Service{db: db, queue: q, logger: logger, audit: auditSvc}
}

// RouteOrder processes an incoming order and creates routed orders for each supplier.
func (s *Service) RouteOrder(ctx context.Context, resellerShopID string, orderPayload map[string]interface{}) error {
	orderID, ok := orderPayload["id"].(float64)
	if !ok {
		return fmt.Errorf("invalid order ID in payload")
	}
	orderIDInt := int64(orderID)

	// Idempotency check
	idemKey := idempotency.GenerateKey("route_order", resellerShopID, strconv.FormatInt(orderIDInt, 10))

	lineItems, ok := orderPayload["line_items"].([]interface{})
	if !ok {
		return fmt.Errorf("no line items in order")
	}

	// Group line items by supplier
	type supplierItem struct {
		ResellerLineItemID int64
		SupplierVariantID  int64
		ResellerVariantID  int64
		SupplierShopID     string
		Title              string
		SKU                string
		Quantity           int
		WholesalePrice     float64
	}

	supplierGroups := make(map[string][]supplierItem)

	for _, li := range lineItems {
		item, ok := li.(map[string]interface{})
		if !ok {
			continue
		}

		variantID, ok := item["variant_id"].(float64)
		if !ok {
			continue
		}
		resellerVariantID := int64(variantID)
		lineItemID := int64(item["id"].(float64))
		quantity := int(item["quantity"].(float64))
		title, _ := item["title"].(string)
		sku, _ := item["sku"].(string)

		// Look up product link to find supplier
		var supplierShopID string
		var supplierVariantID int64
		var wholesalePrice float64
		err := s.db.QueryRow(ctx, `
			SELECT pl.supplier_shop_id, pl.supplier_variant_id, COALESCE(slv.wholesale_price, 0)
			FROM product_links pl
			LEFT JOIN supplier_listing_variants slv ON slv.shopify_variant_id = pl.supplier_variant_id
			WHERE pl.reseller_shop_id = $1 AND pl.reseller_variant_id = $2 AND pl.is_active = TRUE
		`, resellerShopID, resellerVariantID).Scan(&supplierShopID, &supplierVariantID, &wholesalePrice)
		if err != nil {
			// Not a linked product, skip
			s.logger.Debug().Int64("variant_id", resellerVariantID).Msg("no product link for variant, skipping")
			continue
		}

		supplierGroups[supplierShopID] = append(supplierGroups[supplierShopID], supplierItem{
			ResellerLineItemID: lineItemID,
			SupplierVariantID:  supplierVariantID,
			ResellerVariantID:  resellerVariantID,
			SupplierShopID:     supplierShopID,
			Title:              title,
			SKU:                sku,
			Quantity:           quantity,
			WholesalePrice:     wholesalePrice,
		})
	}

	if len(supplierGroups) == 0 {
		s.logger.Info().Int64("order_id", orderIDInt).Msg("no linked products in order")
		return nil
	}

	// Extract shipping info
	shippingAddr, _ := json.Marshal(orderPayload["shipping_address"])
	shippingAddrMap, _ := orderPayload["shipping_address"].(map[string]interface{})
	customerName := ""
	if shippingAddrMap != nil {
		firstName, _ := shippingAddrMap["first_name"].(string)
		lastName, _ := shippingAddrMap["last_name"].(string)
		customerName = firstName + " " + lastName
	}
	customerEmail, _ := orderPayload["email"].(string)
	customerPhone, _ := orderPayload["phone"].(string)
	orderNumber, _ := orderPayload["name"].(string)
	currency, _ := orderPayload["currency"].(string)

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	for supplierShopID, items := range supplierGroups {
		var totalWholesale float64
		for _, item := range items {
			totalWholesale += item.WholesalePrice * float64(item.Quantity)
		}

		// ==========================================
		// STOCK VALIDATION (post-purchase queue)
		// ==========================================
		// Check supplier stock INSIDE transaction with row lock
		// to prevent concurrent overselling
		stockSufficient := true
		stockFailureReason := ""

		for _, item := range items {
			var availableQty int
			var stockPct int

			// Lock the variant row to prevent concurrent reads
			err := tx.QueryRow(ctx, `
				SELECT COALESCE(slv.inventory_quantity, 0), COALESCE(sl.marketplace_stock_percent, 100)
				FROM supplier_listing_variants slv
				JOIN supplier_listings sl ON sl.id = slv.listing_id
				WHERE slv.shopify_variant_id = $1 AND sl.supplier_shop_id = $2
				FOR UPDATE
			`, item.SupplierVariantID, supplierShopID).Scan(&availableQty, &stockPct)

			if err != nil {
				// Can't verify stock — proceed with routing (fail-open)
				s.logger.Warn().Err(err).Int64("variant", item.SupplierVariantID).Msg("stock check failed, proceeding anyway")
				continue
			}

			// Calculate effective available with marketplace allocation
			effectiveAvailable := (availableQty * stockPct) / 100

			if item.Quantity > effectiveAvailable {
				stockSufficient = false
				stockFailureReason = fmt.Sprintf("Insufficient stock for '%s': requested %d, available %d (of %d at %d%%)",
					item.Title, item.Quantity, effectiveAvailable, availableQty, stockPct)
				s.logger.Warn().
					Str("supplier", supplierShopID).
					Str("title", item.Title).
					Int("requested", item.Quantity).
					Int("available", effectiveAvailable).
					Msg("stock validation failed")
				break
			}
		}

		// Determine order status based on stock validation
		orderStatus := "pending"
		if !stockSufficient {
			orderStatus = "cancelled"
		}

		var routedOrderID string
		err := tx.QueryRow(ctx, `
			INSERT INTO routed_orders (reseller_shop_id, supplier_shop_id, reseller_order_id, reseller_order_number,
				status, customer_shipping_name, customer_shipping_address, customer_email, customer_phone,
				total_wholesale_amount, currency, idempotency_key, stock_validated, stock_failure_reason)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
			ON CONFLICT (idempotency_key) DO NOTHING
			RETURNING id
		`, resellerShopID, supplierShopID, orderIDInt, orderNumber,
			orderStatus, customerName, shippingAddr, customerEmail, customerPhone,
			totalWholesale, currency,
			idemKey+":"+supplierShopID,
			stockSufficient, stockFailureReason,
		).Scan(&routedOrderID)
		if err != nil {
			s.logger.Info().Str("idempotency_key", idemKey).Msg("order already routed (idempotent)")
			continue
		}

		for _, item := range items {
			_, err = tx.Exec(ctx, `
				INSERT INTO routed_order_items (routed_order_id, reseller_line_item_id, supplier_variant_id,
					reseller_variant_id, title, sku, quantity, wholesale_unit_price)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			`, routedOrderID, item.ResellerLineItemID, item.SupplierVariantID,
				item.ResellerVariantID, item.Title, item.SKU, item.Quantity, item.WholesalePrice)
			if err != nil {
				return fmt.Errorf("insert routed item: %w", err)
			}
		}

		if stockSufficient {
			// STOCK AVAILABLE — reserve stock by decrementing inventory
			for _, item := range items {
				tx.Exec(ctx, `
					UPDATE supplier_listing_variants
					SET inventory_quantity = GREATEST(0, inventory_quantity - $1)
					WHERE shopify_variant_id = $2
				`, item.Quantity, item.SupplierVariantID)
			}

			// Queue notification to supplier (only for valid orders)
			_, err = s.queue.Enqueue(ctx, "notifications", "supplier_new_order", map[string]string{
				"routed_order_id": routedOrderID,
				"supplier_shop_id": supplierShopID,
			}, 3)
			if err != nil {
				s.logger.Error().Err(err).Msg("failed to enqueue supplier notification")
			}

			// Auto-charge reseller for wholesale amount
			s.queue.Enqueue(ctx, "orders", "charge_order", map[string]string{
				"routed_order_id":  routedOrderID,
				"reseller_shop_id": resellerShopID,
				"supplier_shop_id": supplierShopID,
				"wholesale_amount": fmt.Sprintf("%.2f", totalWholesale),
			}, 3)

			s.audit.Log(ctx, resellerShopID, "system", "", "order_routed", "routed_order", routedOrderID,
				map[string]interface{}{"order_id": orderIDInt, "supplier": supplierShopID, "items": len(items), "stock_validated": true}, "success", "")
		} else {
			// STOCK INSUFFICIENT — order created as cancelled, notify reseller
			s.audit.Log(ctx, resellerShopID, "system", "", "order_stock_failed", "routed_order", routedOrderID,
				map[string]interface{}{"order_id": orderIDInt, "reason": stockFailureReason}, "failure", stockFailureReason)

			s.logger.Warn().
				Str("order_id", routedOrderID).
				Str("reason", stockFailureReason).
				Msg("order cancelled due to insufficient stock")

			// Notify reseller about stock failure via notification queue
			s.queue.Enqueue(ctx, "notifications", "supplier_new_order", map[string]string{
				"routed_order_id": routedOrderID,
				"supplier_shop_id": supplierShopID,
				"stock_failed":    "true",
				"reseller_shop_id": resellerShopID,
				"failure_reason":   stockFailureReason,
			}, 1)
		}
	}

	return tx.Commit(ctx)
}

// ListRoutedOrders returns routed orders visible to a shop (supplier or reseller view).
func (s *Service) ListRoutedOrders(ctx context.Context, shopID, role, status string, limit, offset int) ([]RoutedOrder, int, error) {
	shopColumn := "reseller_shop_id"
	if role == "supplier" {
		shopColumn = "supplier_shop_id"
	}

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM routed_orders WHERE %s = $1`, shopColumn)
	listQuery := fmt.Sprintf(`
		SELECT ro.id, ro.reseller_shop_id, ro.supplier_shop_id, ro.reseller_order_id, COALESCE(ro.reseller_order_number,''),
			ro.status, COALESCE(ro.customer_shipping_name,''), ro.customer_shipping_address,
			COALESCE(ro.customer_email,''), COALESCE(ro.customer_phone,''),
			COALESCE(ro.total_wholesale_amount,0), COALESCE(ro.currency,'USD'), COALESCE(ro.notes,''), ro.created_at, ro.updated_at,
			COALESCE(rs.name, rs.shopify_domain, '') as reseller_name,
			COALESCE(ss.name, ss.shopify_domain, '') as supplier_name
		FROM routed_orders ro
		LEFT JOIN shops rs ON rs.id = ro.reseller_shop_id
		LEFT JOIN shops ss ON ss.id = ro.supplier_shop_id
		WHERE ro.%s = $1`, shopColumn)

	args := []interface{}{shopID}
	if status != "" {
		countQuery += ` AND status = $2`
		listQuery += ` AND ro.status = $2`
		args = append(args, status)
	}

	var total int
	if err := s.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count orders: %w", err)
	}

	listQuery += fmt.Sprintf(` ORDER BY created_at DESC LIMIT %d OFFSET %d`, limit, offset)
	rows, err := s.db.Query(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()

	var orders []RoutedOrder
	for rows.Next() {
		var o RoutedOrder
		if err := rows.Scan(&o.ID, &o.ResellerShopID, &o.SupplierShopID, &o.ResellerOrderID,
			&o.ResellerOrderNumber, &o.Status, &o.CustomerShippingName, &o.CustomerShippingAddr,
			&o.CustomerEmail, &o.CustomerPhone, &o.TotalWholesaleAmount, &o.Currency, &o.Notes,
			&o.CreatedAt, &o.UpdatedAt, &o.ResellerShopName, &o.SupplierShopName); err != nil {
			return nil, 0, fmt.Errorf("scan order: %w", err)
		}

		// Load items with product images
		itemRows, err := s.db.Query(ctx, `
			SELECT roi.id, roi.routed_order_id, roi.reseller_line_item_id, roi.supplier_variant_id,
				roi.reseller_variant_id, COALESCE(roi.title,''), COALESCE(roi.sku,''),
				roi.quantity, roi.wholesale_unit_price, roi.fulfillment_status, roi.fulfilled_quantity,
				COALESCE(sl.images, '[]'::jsonb)
			FROM routed_order_items roi
			LEFT JOIN supplier_listing_variants slv ON slv.shopify_variant_id = roi.supplier_variant_id
			LEFT JOIN supplier_listings sl ON sl.id = slv.listing_id
			WHERE roi.routed_order_id = $1
			ORDER BY roi.created_at
		`, o.ID)
		if err == nil {
			for itemRows.Next() {
				var item RoutedOrderItem
				var imagesRaw json.RawMessage
				if err := itemRows.Scan(&item.ID, &item.RoutedOrderID, &item.ResellerLineItemID,
					&item.SupplierVariantID, &item.ResellerVariantID, &item.Title, &item.SKU,
					&item.Quantity, &item.WholesaleUnitPrice, &item.FulfillmentStatus, &item.FulfilledQuantity,
					&imagesRaw); err == nil {
					// Extract first image URL
					var innerStr string
					if json.Unmarshal(imagesRaw, &innerStr) == nil && len(innerStr) > 0 {
						var imgs []struct{ URL string `json:"url"` }
						json.Unmarshal([]byte(innerStr), &imgs)
						if len(imgs) > 0 { item.ImageURL = imgs[0].URL }
					}
					if item.ImageURL == "" {
						var imgs []struct{ URL string `json:"url"` }
						json.Unmarshal(imagesRaw, &imgs)
						if len(imgs) > 0 { item.ImageURL = imgs[0].URL }
					}
					o.Items = append(o.Items, item)
				}
			}
			itemRows.Close()
		}

		orders = append(orders, o)
	}

	return orders, total, nil
}

// GetRoutedOrder returns a single routed order with items.
func (s *Service) GetRoutedOrder(ctx context.Context, orderID, shopID string) (*RoutedOrder, error) {
	var o RoutedOrder
	err := s.db.QueryRow(ctx, `
		SELECT ro.id, ro.reseller_shop_id, ro.supplier_shop_id, ro.reseller_order_id, COALESCE(ro.reseller_order_number,''),
			ro.status, COALESCE(ro.customer_shipping_name,''), ro.customer_shipping_address,
			COALESCE(ro.customer_email,''), COALESCE(ro.customer_phone,''),
			COALESCE(ro.total_wholesale_amount,0), COALESCE(ro.currency,'USD'), COALESCE(ro.notes,''), ro.created_at, ro.updated_at,
			COALESCE(rs.name, rs.shopify_domain, '') as reseller_name,
			COALESCE(ss.name, ss.shopify_domain, '') as supplier_name
		FROM routed_orders ro
		LEFT JOIN shops rs ON rs.id = ro.reseller_shop_id
		LEFT JOIN shops ss ON ss.id = ro.supplier_shop_id
		WHERE ro.id = $1 AND (ro.reseller_shop_id = $2 OR ro.supplier_shop_id = $2)
	`, orderID, shopID).Scan(&o.ID, &o.ResellerShopID, &o.SupplierShopID, &o.ResellerOrderID,
		&o.ResellerOrderNumber, &o.Status, &o.CustomerShippingName, &o.CustomerShippingAddr,
		&o.CustomerEmail, &o.CustomerPhone, &o.TotalWholesaleAmount, &o.Currency, &o.Notes,
		&o.CreatedAt, &o.UpdatedAt, &o.ResellerShopName, &o.SupplierShopName)
	if err != nil {
		return nil, fmt.Errorf("get order: %w", err)
	}

	rows, err := s.db.Query(ctx, `
		SELECT id, routed_order_id, reseller_line_item_id, supplier_variant_id, reseller_variant_id,
			COALESCE(title,''), COALESCE(sku,''), quantity, wholesale_unit_price, fulfillment_status, fulfilled_quantity
		FROM routed_order_items WHERE routed_order_id = $1 ORDER BY created_at
	`, orderID)
	if err != nil {
		return nil, fmt.Errorf("get items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var item RoutedOrderItem
		if err := rows.Scan(&item.ID, &item.RoutedOrderID, &item.ResellerLineItemID,
			&item.SupplierVariantID, &item.ResellerVariantID, &item.Title, &item.SKU,
			&item.Quantity, &item.WholesaleUnitPrice, &item.FulfillmentStatus, &item.FulfilledQuantity); err != nil {
			return nil, fmt.Errorf("scan item: %w", err)
		}
		o.Items = append(o.Items, item)
	}

	return &o, nil
}

// AcceptOrder marks a routed order as accepted by the supplier.
func (s *Service) AcceptOrder(ctx context.Context, orderID, supplierShopID string) error {
	result, err := s.db.Exec(ctx, `
		UPDATE routed_orders SET status = 'accepted', accepted_at = NOW()
		WHERE id = $1 AND supplier_shop_id = $2 AND status = 'pending'
	`, orderID, supplierShopID)
	if err != nil {
		return fmt.Errorf("accept order: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("order not found or not in pending state")
	}

	// Update supplier stats + response time
	s.db.Exec(ctx, `UPDATE supplier_profiles SET total_orders_received = total_orders_received + 1 WHERE shop_id = $1`, supplierShopID)
	// Calculate average response time (hours between order created and accepted)
	s.db.Exec(ctx, `
		UPDATE supplier_profiles SET avg_fulfillment_hours = COALESCE((
			SELECT AVG(EXTRACT(EPOCH FROM (accepted_at - created_at)) / 3600)
			FROM routed_orders WHERE supplier_shop_id = $1 AND accepted_at IS NOT NULL
		), 0) WHERE shop_id = $1
	`, supplierShopID)
	s.UpdateReliabilityScore(ctx, supplierShopID)

	s.audit.Log(ctx, supplierShopID, "merchant", supplierShopID, "order_accepted", "routed_order", orderID, nil, "success", "")
	return nil
}

// RejectOrder marks a routed order as rejected by the supplier and restores inventory.
func (s *Service) RejectOrder(ctx context.Context, orderID, supplierShopID, reason string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	result, err := tx.Exec(ctx, `
		UPDATE routed_orders SET status = 'rejected', rejected_at = NOW(), notes = $3
		WHERE id = $1 AND supplier_shop_id = $2 AND status IN ('pending', 'accepted')
	`, orderID, supplierShopID, reason)
	if err != nil {
		return fmt.Errorf("reject order: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("order not found or not in pending/accepted state")
	}

	// Restore inventory — collect items first, then update (avoid conn busy)
	type restoreItem struct {
		VariantID int64
		Qty       int
	}
	var restoreItems []restoreItem
	rows, err := tx.Query(ctx, `
		SELECT supplier_variant_id, quantity FROM routed_order_items WHERE routed_order_id = $1
	`, orderID)
	if err == nil {
		for rows.Next() {
			var ri restoreItem
			rows.Scan(&ri.VariantID, &ri.Qty)
			restoreItems = append(restoreItems, ri)
		}
		rows.Close()
	}
	for _, ri := range restoreItems {
		_, updateErr := tx.Exec(ctx, `
			UPDATE supplier_listing_variants
			SET inventory_quantity = inventory_quantity + $1
			WHERE shopify_variant_id = $2
		`, ri.Qty, ri.VariantID)
		if updateErr != nil {
			s.logger.Error().Err(updateErr).Int64("variant", ri.VariantID).Int("qty", ri.Qty).Msg("failed to restore inventory")
		} else {
			s.logger.Info().Int64("variant", ri.VariantID).Int("qty", ri.Qty).Msg("inventory restored on reject")
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	// Update supplier stats
	s.db.Exec(ctx, `UPDATE supplier_profiles SET cancellation_count = cancellation_count + 1 WHERE shop_id = $1`, supplierShopID)
	s.UpdateReliabilityScore(ctx, supplierShopID)

	s.audit.Log(ctx, supplierShopID, "merchant", supplierShopID, "order_rejected", "routed_order", orderID,
		map[string]string{"reason": reason}, "success", "")
	return nil
}

// UpdateReliabilityScore recalculates the supplier's reliability score.
// Score = (fulfilled / received * 4.0) + (1 - cancellations/received) * 1.0, max 5.0
func (s *Service) UpdateReliabilityScore(ctx context.Context, supplierShopID string) {
	var received, fulfilled, cancellations int
	s.db.QueryRow(ctx, `SELECT COALESCE(total_orders_received,0), COALESCE(total_orders_fulfilled,0), COALESCE(cancellation_count,0) FROM supplier_profiles WHERE shop_id = $1`, supplierShopID).Scan(&received, &fulfilled, &cancellations)

	if received == 0 {
		return
	}

	fulfillRate := float64(fulfilled) / float64(received)
	cancelRate := float64(cancellations) / float64(received)
	score := (fulfillRate * 4.0) + ((1.0 - cancelRate) * 1.0) // max 5.0
	if score > 5.0 { score = 5.0 }
	if score < 0 { score = 0 }

	s.db.Exec(ctx, `UPDATE supplier_profiles SET reliability_score = $1 WHERE shop_id = $2`, score, supplierShopID)
}
