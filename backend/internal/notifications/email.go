package notifications

import (
	"fmt"
	"net/smtp"
	"os"
	"strings"

	"github.com/rs/zerolog"
)

// Service handles sending notifications.
type Service struct {
	smtpHost string
	smtpPort string
	smtpUser string
	smtpPass string
	fromAddr string
	enabled  bool
	logger   zerolog.Logger
}

// NewService creates a notification service from environment variables.
func NewService(logger zerolog.Logger) *Service {
	host := os.Getenv("SMTP_HOST")
	port := os.Getenv("SMTP_PORT")
	if port == "" {
		port = "587"
	}
	user := os.Getenv("SMTP_USER")
	pass := os.Getenv("SMTP_PASS")
	from := os.Getenv("SMTP_FROM")
	if from == "" {
		from = user
	}

	enabled := host != "" && user != ""
	if !enabled {
		logger.Info().Msg("email notifications disabled (SMTP_HOST/SMTP_USER not set)")
	}

	return &Service{
		smtpHost: host,
		smtpPort: port,
		smtpUser: user,
		smtpPass: pass,
		fromAddr: from,
		enabled:  enabled,
		logger:   logger,
	}
}

// SendOrderNotification sends an email to the supplier about a new order.
func (s *Service) SendOrderNotification(to, supplierName, orderNumber string, amount float64, currency string) error {
	if !s.enabled {
		s.logger.Debug().Str("to", to).Str("order", orderNumber).Msg("email notification skipped (not configured)")
		return nil
	}

	subject := fmt.Sprintf("New Order: %s", orderNumber)
	body := fmt.Sprintf(`Hello %s,

You have received a new order from the DropToDrop network.

Order: %s
Amount: $%.2f %s

Please log in to your DropToDrop dashboard to review and accept this order.

Best regards,
DropToDrop Team
`, supplierName, orderNumber, amount, currency)

	return s.sendEmail(to, subject, body)
}

// SendFulfillmentNotification notifies the reseller that tracking has been added.
func (s *Service) SendFulfillmentNotification(to, orderNumber, trackingNumber, trackingCompany string) error {
	if !s.enabled {
		s.logger.Debug().Str("to", to).Str("order", orderNumber).Msg("fulfillment notification skipped (not configured)")
		return nil
	}

	subject := fmt.Sprintf("Order %s has been shipped", orderNumber)
	body := fmt.Sprintf(`Your order %s has been fulfilled.

Tracking Number: %s
Carrier: %s

The tracking information has been synced to your Shopify store.

Best regards,
DropToDrop Team
`, orderNumber, trackingNumber, trackingCompany)

	return s.sendEmail(to, subject, body)
}

func (s *Service) sendEmail(to, subject, body string) error {
	msg := strings.Join([]string{
		fmt.Sprintf("From: %s", s.fromAddr),
		fmt.Sprintf("To: %s", to),
		fmt.Sprintf("Subject: %s", subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		"",
		body,
	}, "\r\n")

	auth := smtp.PlainAuth("", s.smtpUser, s.smtpPass, s.smtpHost)
	addr := fmt.Sprintf("%s:%s", s.smtpHost, s.smtpPort)

	if err := smtp.SendMail(addr, auth, s.fromAddr, []string{to}, []byte(msg)); err != nil {
		s.logger.Error().Err(err).Str("to", to).Str("subject", subject).Msg("failed to send email")
		return fmt.Errorf("send email: %w", err)
	}

	s.logger.Info().Str("to", to).Str("subject", subject).Msg("email sent")
	return nil
}

// IsEnabled returns whether the notification service is configured.
func (s *Service) IsEnabled() bool {
	return s.enabled
}
