package orders

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

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
	Items                  []RoutedOrderItem `json:"items,omitempty"`
	CreatedAt              string            `json:"created_at"`
	UpdatedAt              string            `json:"updated_at"`
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

		var routedOrderID string
		err := tx.QueryRow(ctx, `
			INSERT INTO routed_orders (reseller_shop_id, supplier_shop_id, reseller_order_id, reseller_order_number,
				status, customer_shipping_name, customer_shipping_address, customer_email, customer_phone,
				total_wholesale_amount, currency, idempotency_key)
			VALUES ($1, $2, $3, $4, 'pending', $5, $6, $7, $8, $9, $10, $11)
			ON CONFLICT (idempotency_key) DO NOTHING
			RETURNING id
		`, resellerShopID, supplierShopID, orderIDInt, orderNumber,
			customerName, shippingAddr, customerEmail, customerPhone,
			totalWholesale, currency,
			idemKey+":"+supplierShopID,
		).Scan(&routedOrderID)
		if err != nil {
			// Likely idempotency conflict - this is expected behavior
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

		// Queue notification to supplier
		_, err = s.queue.Enqueue(ctx, "notifications", "supplier_new_order", map[string]string{
			"routed_order_id": routedOrderID,
			"supplier_shop_id": supplierShopID,
		}, 3)
		if err != nil {
			s.logger.Error().Err(err).Msg("failed to enqueue supplier notification")
		}

		s.audit.Log(ctx, resellerShopID, "system", "", "order_routed", "routed_order", routedOrderID,
			map[string]interface{}{"order_id": orderIDInt, "supplier": supplierShopID, "items": len(items)}, "success", "")
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
		SELECT id, reseller_shop_id, supplier_shop_id, reseller_order_id, COALESCE(reseller_order_number,''),
			status, COALESCE(customer_shipping_name,''), customer_shipping_address,
			COALESCE(customer_email,''), COALESCE(customer_phone,''),
			COALESCE(total_wholesale_amount,0), COALESCE(currency,'USD'), COALESCE(notes,''), created_at, updated_at
		FROM routed_orders WHERE %s = $1`, shopColumn)

	args := []interface{}{shopID}
	if status != "" {
		countQuery += ` AND status = $2`
		listQuery += ` AND status = $2`
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
			&o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan order: %w", err)
		}
		orders = append(orders, o)
	}

	return orders, total, nil
}

// GetRoutedOrder returns a single routed order with items.
func (s *Service) GetRoutedOrder(ctx context.Context, orderID, shopID string) (*RoutedOrder, error) {
	var o RoutedOrder
	err := s.db.QueryRow(ctx, `
		SELECT id, reseller_shop_id, supplier_shop_id, reseller_order_id, COALESCE(reseller_order_number,''),
			status, COALESCE(customer_shipping_name,''), customer_shipping_address,
			COALESCE(customer_email,''), COALESCE(customer_phone,''),
			COALESCE(total_wholesale_amount,0), COALESCE(currency,'USD'), COALESCE(notes,''), created_at, updated_at
		FROM routed_orders WHERE id = $1 AND (reseller_shop_id = $2 OR supplier_shop_id = $2)
	`, orderID, shopID).Scan(&o.ID, &o.ResellerShopID, &o.SupplierShopID, &o.ResellerOrderID,
		&o.ResellerOrderNumber, &o.Status, &o.CustomerShippingName, &o.CustomerShippingAddr,
		&o.CustomerEmail, &o.CustomerPhone, &o.TotalWholesaleAmount, &o.Currency, &o.Notes,
		&o.CreatedAt, &o.UpdatedAt)
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

	s.audit.Log(ctx, supplierShopID, "merchant", supplierShopID, "order_accepted", "routed_order", orderID, nil, "success", "")
	return nil
}

// RejectOrder marks a routed order as rejected by the supplier.
func (s *Service) RejectOrder(ctx context.Context, orderID, supplierShopID, reason string) error {
	result, err := s.db.Exec(ctx, `
		UPDATE routed_orders SET status = 'rejected', rejected_at = NOW(), notes = $3
		WHERE id = $1 AND supplier_shop_id = $2 AND status = 'pending'
	`, orderID, supplierShopID, reason)
	if err != nil {
		return fmt.Errorf("reject order: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("order not found or not in pending state")
	}

	s.audit.Log(ctx, supplierShopID, "merchant", supplierShopID, "order_rejected", "routed_order", orderID,
		map[string]string{"reason": reason}, "success", "")
	return nil
}
