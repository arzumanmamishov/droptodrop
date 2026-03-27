package inappnotif

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

// Notification represents an in-app notification.
type Notification struct {
	ID        string    `json:"id"`
	ShopID    string    `json:"shop_id"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Type      string    `json:"type"`
	IsRead    bool      `json:"is_read"`
	Link      *string   `json:"link"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateInput holds the fields needed to create a notification.
type CreateInput struct {
	ShopID  string  `json:"shop_id"`
	Title   string  `json:"title"`
	Message string  `json:"message"`
	Type    string  `json:"type"`
	Link    *string `json:"link"`
}

// Service handles in-app notification operations.
type Service struct {
	db     *pgxpool.Pool
	logger zerolog.Logger
}

// NewService creates a new in-app notification service.
func NewService(db *pgxpool.Pool, logger zerolog.Logger) *Service {
	return &Service{db: db, logger: logger}
}

// Create creates a new in-app notification.
func (s *Service) Create(ctx context.Context, input CreateInput) (*Notification, error) {
	if input.Type == "" {
		input.Type = "info"
	}

	n := &Notification{}
	err := s.db.QueryRow(ctx, `
		INSERT INTO notifications (shop_id, title, message, type, link)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, shop_id, title, message, type, is_read, link, created_at
	`, input.ShopID, input.Title, input.Message, input.Type, input.Link).Scan(
		&n.ID, &n.ShopID, &n.Title, &n.Message, &n.Type, &n.IsRead, &n.Link, &n.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create notification: %w", err)
	}
	return n, nil
}

// List returns notifications for a shop, ordered by unread first then by creation date descending.
func (s *Service) List(ctx context.Context, shopID string, limit, offset int) ([]Notification, int, error) {
	var total int
	err := s.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM notifications WHERE shop_id = $1
	`, shopID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count notifications: %w", err)
	}

	rows, err := s.db.Query(ctx, `
		SELECT id, shop_id, title, message, type, is_read, link, created_at
		FROM notifications
		WHERE shop_id = $1
		ORDER BY is_read ASC, created_at DESC
		LIMIT $2 OFFSET $3
	`, shopID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()

	var notifs []Notification
	for rows.Next() {
		var n Notification
		if err := rows.Scan(
			&n.ID, &n.ShopID, &n.Title, &n.Message, &n.Type, &n.IsRead, &n.Link, &n.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan notification: %w", err)
		}
		notifs = append(notifs, n)
	}

	return notifs, total, nil
}

// MarkRead marks a single notification as read.
func (s *Service) MarkRead(ctx context.Context, notifID, shopID string) error {
	tag, err := s.db.Exec(ctx, `
		UPDATE notifications SET is_read = TRUE
		WHERE id = $1 AND shop_id = $2
	`, notifID, shopID)
	if err != nil {
		return fmt.Errorf("mark read: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("notification not found")
	}
	return nil
}

// MarkAllRead marks all notifications for a shop as read.
func (s *Service) MarkAllRead(ctx context.Context, shopID string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE notifications SET is_read = TRUE
		WHERE shop_id = $1 AND is_read = FALSE
	`, shopID)
	if err != nil {
		return fmt.Errorf("mark all read: %w", err)
	}
	return nil
}

// CountUnread returns the number of unread notifications for a shop.
func (s *Service) CountUnread(ctx context.Context, shopID string) (int, error) {
	var count int
	err := s.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM notifications WHERE shop_id = $1 AND is_read = FALSE
	`, shopID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count unread: %w", err)
	}
	return count, nil
}
