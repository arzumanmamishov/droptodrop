package hmac

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
)

// VerifyWebhook verifies a Shopify webhook HMAC signature using the raw request body.
// This MUST use the raw body bytes, not re-serialized JSON.
func VerifyWebhook(rawBody []byte, secret string, signatureHeader string) bool {
	if len(rawBody) == 0 || secret == "" || signatureHeader == "" {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(rawBody)
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(signatureHeader))
}

// VerifyProxy verifies Shopify app proxy request signatures.
func VerifyProxy(queryString string, secret string, signature string) bool {
	if secret == "" || signature == "" {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(queryString))
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(signature))
}

// ExtractRawBody reads the full request body and returns it as bytes.
// The caller is responsible for restoring the body if needed.
func ExtractRawBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, fmt.Errorf("request body is nil")
	}
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	return body, nil
}

// ComputeHMAC computes HMAC-SHA256 and returns base64-encoded result.
func ComputeHMAC(message []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(message)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
