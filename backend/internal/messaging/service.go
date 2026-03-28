package messaging

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

type Conversation struct {
	ID              string    `json:"id"`
	SupplierShopID  string    `json:"supplier_shop_id"`
	ResellerShopID  string    `json:"reseller_shop_id"`
	Subject         string    `json:"subject"`
	LastMessageAt   time.Time `json:"last_message_at"`
	CreatedAt       time.Time `json:"created_at"`
	OtherShopName   string    `json:"other_shop_name,omitempty"`
	UnreadCount     int       `json:"unread_count"`
	LastMessage     string    `json:"last_message,omitempty"`
}

type Message struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	SenderShopID   string    `json:"sender_shop_id"`
	Content        string    `json:"content"`
	IsRead         bool      `json:"is_read"`
	CreatedAt      time.Time `json:"created_at"`
	IsMine         bool      `json:"is_mine"`
}

type OrderComment struct {
	ID             string    `json:"id"`
	RoutedOrderID  string    `json:"routed_order_id"`
	ShopID         string    `json:"shop_id"`
	ShopRole       string    `json:"shop_role"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"created_at"`
}

type Announcement struct {
	ID              string    `json:"id"`
	SupplierShopID  string    `json:"supplier_shop_id"`
	Title           string    `json:"title"`
	Content         string    `json:"content"`
	IsPinned        bool      `json:"is_pinned"`
	CreatedAt       time.Time `json:"created_at"`
	IsRead          bool      `json:"is_read"`
	SupplierName    string    `json:"supplier_name,omitempty"`
}

type Service struct {
	db     *pgxpool.Pool
	logger zerolog.Logger
}

func NewService(db *pgxpool.Pool, logger zerolog.Logger) *Service {
	return &Service{db: db, logger: logger}
}

// ---- CONVERSATIONS ----

func (s *Service) GetOrCreateConversation(ctx context.Context, supplierShopID, resellerShopID, subject string) (*Conversation, error) {
	var conv Conversation
	err := s.db.QueryRow(ctx, `
		INSERT INTO conversations (supplier_shop_id, reseller_shop_id, subject)
		VALUES ($1, $2, $3)
		ON CONFLICT (supplier_shop_id, reseller_shop_id) DO UPDATE SET last_message_at = NOW()
		RETURNING id, supplier_shop_id, reseller_shop_id, subject, last_message_at, created_at
	`, supplierShopID, resellerShopID, subject).Scan(
		&conv.ID, &conv.SupplierShopID, &conv.ResellerShopID, &conv.Subject, &conv.LastMessageAt, &conv.CreatedAt)
	return &conv, err
}

func (s *Service) ListConversations(ctx context.Context, shopID string) ([]Conversation, error) {
	rows, err := s.db.Query(ctx, `
		SELECT c.id, c.supplier_shop_id, c.reseller_shop_id, c.subject, c.last_message_at, c.created_at,
			CASE WHEN c.supplier_shop_id = $1 THEN COALESCE(rs.shopify_domain,'') ELSE COALESCE(ss.shopify_domain,'') END as other_name,
			(SELECT COUNT(*) FROM messages m WHERE m.conversation_id = c.id AND m.sender_shop_id != $1 AND m.is_read = FALSE) as unread,
			COALESCE((SELECT content FROM messages m WHERE m.conversation_id = c.id ORDER BY m.created_at DESC LIMIT 1), '') as last_msg
		FROM conversations c
		LEFT JOIN shops ss ON ss.id = c.supplier_shop_id
		LEFT JOIN shops rs ON rs.id = c.reseller_shop_id
		WHERE c.supplier_shop_id = $1 OR c.reseller_shop_id = $1
		ORDER BY c.last_message_at DESC
	`, shopID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var convs []Conversation
	for rows.Next() {
		var c Conversation
		if err := rows.Scan(&c.ID, &c.SupplierShopID, &c.ResellerShopID, &c.Subject, &c.LastMessageAt, &c.CreatedAt, &c.OtherShopName, &c.UnreadCount, &c.LastMessage); err != nil {
			return nil, err
		}
		convs = append(convs, c)
	}
	return convs, nil
}

func (s *Service) SendMessage(ctx context.Context, conversationID, senderShopID, content string) (*Message, error) {
	// Verify sender is part of conversation
	var count int
	s.db.QueryRow(ctx, `SELECT COUNT(*) FROM conversations WHERE id = $1 AND (supplier_shop_id = $2 OR reseller_shop_id = $2)`, conversationID, senderShopID).Scan(&count)
	if count == 0 {
		return nil, nil
	}

	var msg Message
	err := s.db.QueryRow(ctx, `
		INSERT INTO messages (conversation_id, sender_shop_id, content)
		VALUES ($1, $2, $3)
		RETURNING id, conversation_id, sender_shop_id, content, is_read, created_at
	`, conversationID, senderShopID, content).Scan(&msg.ID, &msg.ConversationID, &msg.SenderShopID, &msg.Content, &msg.IsRead, &msg.CreatedAt)
	if err != nil {
		return nil, err
	}

	// Update last_message_at
	s.db.Exec(ctx, `UPDATE conversations SET last_message_at = NOW() WHERE id = $1`, conversationID)

	return &msg, nil
}

func (s *Service) GetMessages(ctx context.Context, conversationID, shopID string, limit int) ([]Message, error) {
	// Mark messages as read
	s.db.Exec(ctx, `UPDATE messages SET is_read = TRUE WHERE conversation_id = $1 AND sender_shop_id != $2 AND is_read = FALSE`, conversationID, shopID)

	rows, err := s.db.Query(ctx, `
		SELECT id, conversation_id, sender_shop_id, content, is_read, created_at
		FROM messages WHERE conversation_id = $1
		ORDER BY created_at ASC LIMIT $2
	`, conversationID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.SenderShopID, &m.Content, &m.IsRead, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.IsMine = m.SenderShopID == shopID
		msgs = append(msgs, m)
	}
	return msgs, nil
}

// ---- ORDER COMMENTS ----

func (s *Service) AddOrderComment(ctx context.Context, orderID, shopID, shopRole, content string) (*OrderComment, error) {
	var comment OrderComment
	err := s.db.QueryRow(ctx, `
		INSERT INTO order_comments (routed_order_id, shop_id, shop_role, content)
		VALUES ($1, $2, $3, $4)
		RETURNING id, routed_order_id, shop_id, shop_role, content, created_at
	`, orderID, shopID, shopRole, content).Scan(&comment.ID, &comment.RoutedOrderID, &comment.ShopID, &comment.ShopRole, &comment.Content, &comment.CreatedAt)
	return &comment, err
}

func (s *Service) ListOrderComments(ctx context.Context, orderID string) ([]OrderComment, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, routed_order_id, shop_id, shop_role, content, created_at
		FROM order_comments WHERE routed_order_id = $1
		ORDER BY created_at ASC
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []OrderComment
	for rows.Next() {
		var c OrderComment
		if err := rows.Scan(&c.ID, &c.RoutedOrderID, &c.ShopID, &c.ShopRole, &c.Content, &c.CreatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	return comments, nil
}

// ---- ANNOUNCEMENTS ----

func (s *Service) CreateAnnouncement(ctx context.Context, supplierShopID, title, content string, isPinned bool) (*Announcement, error) {
	var ann Announcement
	err := s.db.QueryRow(ctx, `
		INSERT INTO announcements (supplier_shop_id, title, content, is_pinned)
		VALUES ($1, $2, $3, $4)
		RETURNING id, supplier_shop_id, title, content, is_pinned, created_at
	`, supplierShopID, title, content, isPinned).Scan(&ann.ID, &ann.SupplierShopID, &ann.Title, &ann.Content, &ann.IsPinned, &ann.CreatedAt)
	return &ann, err
}

func (s *Service) ListAnnouncements(ctx context.Context, shopID, role string) ([]Announcement, error) {
	var query string
	var args []interface{}

	if role == "supplier" {
		query = `SELECT a.id, a.supplier_shop_id, a.title, a.content, a.is_pinned, a.created_at, TRUE as is_read, ''
			FROM announcements a WHERE a.supplier_shop_id = $1
			ORDER BY a.is_pinned DESC, a.created_at DESC LIMIT 50`
		args = []interface{}{shopID}
	} else {
		// Reseller sees announcements from suppliers they've imported from
		query = `SELECT DISTINCT a.id, a.supplier_shop_id, a.title, a.content, a.is_pinned, a.created_at,
				CASE WHEN ar.id IS NOT NULL THEN TRUE ELSE FALSE END as is_read,
				COALESCE(s.shopify_domain, '')
			FROM announcements a
			JOIN supplier_listings sl ON sl.supplier_shop_id = a.supplier_shop_id AND sl.status = 'active'
			JOIN reseller_imports ri ON ri.supplier_listing_id = sl.id AND ri.reseller_shop_id = $1
			LEFT JOIN announcement_reads ar ON ar.announcement_id = a.id AND ar.shop_id = $1
			LEFT JOIN shops s ON s.id = a.supplier_shop_id
			ORDER BY a.is_pinned DESC, a.created_at DESC LIMIT 50`
		args = []interface{}{shopID}
	}

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var anns []Announcement
	for rows.Next() {
		var a Announcement
		if err := rows.Scan(&a.ID, &a.SupplierShopID, &a.Title, &a.Content, &a.IsPinned, &a.CreatedAt, &a.IsRead, &a.SupplierName); err != nil {
			return nil, err
		}
		anns = append(anns, a)
	}
	return anns, nil
}

func (s *Service) MarkAnnouncementRead(ctx context.Context, announcementID, shopID string) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO announcement_reads (announcement_id, shop_id) VALUES ($1, $2)
		ON CONFLICT (announcement_id, shop_id) DO NOTHING
	`, announcementID, shopID)
	return err
}

func (s *Service) DeleteAnnouncement(ctx context.Context, announcementID, supplierShopID string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM announcements WHERE id = $1 AND supplier_shop_id = $2`, announcementID, supplierShopID)
	return err
}
