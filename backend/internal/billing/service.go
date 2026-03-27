package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

type Plan struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	PriceMonthly    float64 `json:"price_monthly"`
	Currency        string  `json:"currency"`
	MaxProducts     int     `json:"max_products"`
	MaxOrdersMonth  int     `json:"max_orders_monthly"`
	MaxSuppliers    int     `json:"max_suppliers"`
	AppFeePercent   float64 `json:"app_fee_percent"`
	TrialDays       int     `json:"trial_days"`
}

type Subscription struct {
	ID                 string     `json:"id"`
	ShopID             string     `json:"shop_id"`
	PlanID             string     `json:"plan_id"`
	PlanName           string     `json:"plan_name"`
	Status             string     `json:"status"`
	TrialEndsAt        *time.Time `json:"trial_ends_at,omitempty"`
	CurrentPeriodStart *time.Time `json:"current_period_start,omitempty"`
	CurrentPeriodEnd   *time.Time `json:"current_period_end,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
}

type UsageRecord struct {
	ID            string    `json:"id"`
	ShopID        string    `json:"shop_id"`
	OrderAmount   float64   `json:"order_amount"`
	FeeAmount     float64   `json:"fee_amount"`
	FeePercent    float64   `json:"fee_percent"`
	CreatedAt     time.Time `json:"created_at"`
}

type UsageSummary struct {
	OrderCount   int     `json:"order_count"`
	ProductCount int     `json:"product_count"`
	TotalFees    float64 `json:"total_fees"`
	Month        string  `json:"month"`
}

type BillingStatus struct {
	HasSubscription bool          `json:"has_subscription"`
	Subscription    *Subscription `json:"subscription,omitempty"`
	Plan            *Plan         `json:"plan,omitempty"`
	Usage           *UsageSummary `json:"usage,omitempty"`
	Limits          *PlanLimits   `json:"limits,omitempty"`
}

type PlanLimits struct {
	MaxProducts      int  `json:"max_products"`
	MaxOrdersMonthly int  `json:"max_orders_monthly"`
	MaxSuppliers     int  `json:"max_suppliers"`
	ProductsUsed     int  `json:"products_used"`
	OrdersThisMonth  int  `json:"orders_this_month"`
	IsOverLimit      bool `json:"is_over_limit"`
}

type Service struct {
	db     *pgxpool.Pool
	logger zerolog.Logger
}

func NewService(db *pgxpool.Pool, logger zerolog.Logger) *Service {
	return &Service{db: db, logger: logger}
}

// ListPlans returns all active billing plans.
func (s *Service) ListPlans(ctx context.Context) ([]Plan, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, name, price_monthly, currency, max_products, max_orders_monthly, max_suppliers, app_fee_percent, trial_days
		FROM billing_plans WHERE is_active = TRUE ORDER BY price_monthly
	`)
	if err != nil {
		return nil, fmt.Errorf("list plans: %w", err)
	}
	defer rows.Close()

	var plans []Plan
	for rows.Next() {
		var p Plan
		if err := rows.Scan(&p.ID, &p.Name, &p.PriceMonthly, &p.Currency, &p.MaxProducts, &p.MaxOrdersMonth, &p.MaxSuppliers, &p.AppFeePercent, &p.TrialDays); err != nil {
			return nil, fmt.Errorf("scan plan: %w", err)
		}
		plans = append(plans, p)
	}
	return plans, nil
}

// GetStatus returns the full billing status for a shop.
func (s *Service) GetStatus(ctx context.Context, shopID string) (*BillingStatus, error) {
	status := &BillingStatus{}

	// Get active subscription
	var sub Subscription
	err := s.db.QueryRow(ctx, `
		SELECT ss.id, ss.shop_id, ss.plan_id, bp.name, ss.status, ss.trial_ends_at,
			ss.current_period_start, ss.current_period_end, ss.created_at
		FROM shop_subscriptions ss
		JOIN billing_plans bp ON bp.id = ss.plan_id
		WHERE ss.shop_id = $1 AND ss.status IN ('active', 'pending')
		ORDER BY ss.created_at DESC LIMIT 1
	`, shopID).Scan(&sub.ID, &sub.ShopID, &sub.PlanID, &sub.PlanName, &sub.Status,
		&sub.TrialEndsAt, &sub.CurrentPeriodStart, &sub.CurrentPeriodEnd, &sub.CreatedAt)
	if err == nil {
		status.HasSubscription = true
		status.Subscription = &sub

		// Get plan details
		var plan Plan
		if err := s.db.QueryRow(ctx, `
			SELECT id, name, price_monthly, currency, max_products, max_orders_monthly, max_suppliers, app_fee_percent, trial_days
			FROM billing_plans WHERE id = $1
		`, sub.PlanID).Scan(&plan.ID, &plan.Name, &plan.PriceMonthly, &plan.Currency, &plan.MaxProducts, &plan.MaxOrdersMonth, &plan.MaxSuppliers, &plan.AppFeePercent, &plan.TrialDays); err == nil {
			status.Plan = &plan
		}
	}

	// Get current month usage
	currentMonth := time.Now().Format("2006-01")
	var usage UsageSummary
	usage.Month = currentMonth
	s.db.QueryRow(ctx, `
		SELECT COALESCE(order_count,0), COALESCE(product_count,0), COALESCE(total_fees,0)
		FROM usage_summaries WHERE shop_id = $1 AND month = $2
	`, shopID, currentMonth).Scan(&usage.OrderCount, &usage.ProductCount, &usage.TotalFees)
	status.Usage = &usage

	// Calculate limits
	if status.Plan != nil {
		limits := &PlanLimits{
			MaxProducts:      status.Plan.MaxProducts,
			MaxOrdersMonthly: status.Plan.MaxOrdersMonth,
			MaxSuppliers:     status.Plan.MaxSuppliers,
			OrdersThisMonth:  usage.OrderCount,
		}

		// Count products
		s.db.QueryRow(ctx, `SELECT COUNT(*) FROM supplier_listings WHERE supplier_shop_id = $1 AND status = 'active'`, shopID).Scan(&limits.ProductsUsed)

		// Check over limit (-1 means unlimited)
		if limits.MaxProducts > 0 && limits.ProductsUsed >= limits.MaxProducts {
			limits.IsOverLimit = true
		}
		if limits.MaxOrdersMonthly > 0 && limits.OrdersThisMonth >= limits.MaxOrdersMonthly {
			limits.IsOverLimit = true
		}

		status.Limits = limits
	}

	return status, nil
}

// Subscribe creates a new subscription for a shop.
func (s *Service) Subscribe(ctx context.Context, shopID, planID string) (*Subscription, error) {
	// Verify plan exists
	var plan Plan
	err := s.db.QueryRow(ctx, `
		SELECT id, name, trial_days FROM billing_plans WHERE id = $1 AND is_active = TRUE
	`, planID).Scan(&plan.ID, &plan.Name, &plan.TrialDays)
	if err != nil {
		return nil, fmt.Errorf("plan not found: %w", err)
	}

	// Cancel existing subscription
	s.db.Exec(ctx, `UPDATE shop_subscriptions SET status = 'cancelled', updated_at = NOW() WHERE shop_id = $1 AND status IN ('active', 'pending')`, shopID)

	now := time.Now()
	trialEnd := now.Add(time.Duration(plan.TrialDays) * 24 * time.Hour)
	periodEnd := now.Add(30 * 24 * time.Hour)

	var sub Subscription
	err = s.db.QueryRow(ctx, `
		INSERT INTO shop_subscriptions (shop_id, plan_id, status, trial_ends_at, current_period_start, current_period_end)
		VALUES ($1, $2, 'active', $3, $4, $5)
		RETURNING id, shop_id, plan_id, status, trial_ends_at, current_period_start, current_period_end, created_at
	`, shopID, planID, trialEnd, now, periodEnd).Scan(
		&sub.ID, &sub.ShopID, &sub.PlanID, &sub.Status,
		&sub.TrialEndsAt, &sub.CurrentPeriodStart, &sub.CurrentPeriodEnd, &sub.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create subscription: %w", err)
	}

	sub.PlanName = plan.Name
	s.logger.Info().Str("shop_id", shopID).Str("plan", planID).Msg("subscription created")
	return &sub, nil
}

// TrackUsage records usage for an order and updates monthly summary.
func (s *Service) TrackUsage(ctx context.Context, shopID, orderID string, orderAmount float64) error {
	// Get fee percent from plan
	feePercent := 2.0
	s.db.QueryRow(ctx, `
		SELECT bp.app_fee_percent FROM shop_subscriptions ss
		JOIN billing_plans bp ON bp.id = ss.plan_id
		WHERE ss.shop_id = $1 AND ss.status = 'active'
	`, shopID).Scan(&feePercent)

	feeAmount := orderAmount * feePercent / 100

	_, err := s.db.Exec(ctx, `
		INSERT INTO usage_records (shop_id, routed_order_id, order_amount, fee_amount, fee_percent)
		VALUES ($1, $2, $3, $4, $5)
	`, shopID, orderID, orderAmount, feeAmount, feePercent)
	if err != nil {
		return fmt.Errorf("track usage: %w", err)
	}

	// Update monthly summary
	month := time.Now().Format("2006-01")
	_, err = s.db.Exec(ctx, `
		INSERT INTO usage_summaries (shop_id, month, order_count, total_fees)
		VALUES ($1, $2, 1, $3)
		ON CONFLICT (shop_id, month) DO UPDATE SET
			order_count = usage_summaries.order_count + 1,
			total_fees = usage_summaries.total_fees + $3,
			updated_at = NOW()
	`, shopID, month, feeAmount)
	if err != nil {
		return fmt.Errorf("update usage summary: %w", err)
	}

	return nil
}

// CancelSubscription cancels a shop's subscription.
func (s *Service) CancelSubscription(ctx context.Context, shopID string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE shop_subscriptions SET status = 'cancelled', updated_at = NOW()
		WHERE shop_id = $1 AND status IN ('active', 'pending')
	`, shopID)
	if err != nil {
		return fmt.Errorf("cancel subscription: %w", err)
	}
	s.logger.Info().Str("shop_id", shopID).Msg("subscription cancelled")
	return nil
}

// Note: Handler is defined in handler.go
