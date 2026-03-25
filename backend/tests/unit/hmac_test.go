package unit

import (
	"testing"

	"github.com/stretchr/testify/assert"

	hmacpkg "github.com/droptodrop/droptodrop/pkg/hmac"
)

func TestVerifyWebhook_ValidSignature(t *testing.T) {
	secret := "test_secret"
	body := []byte(`{"id": 123, "email": "test@example.com"}`)

	// Compute the expected HMAC
	expected := hmacpkg.ComputeHMAC(body, secret)

	// Verify should pass
	assert.True(t, hmacpkg.VerifyWebhook(body, secret, expected))
}

func TestVerifyWebhook_InvalidSignature(t *testing.T) {
	secret := "test_secret"
	body := []byte(`{"id": 123}`)

	assert.False(t, hmacpkg.VerifyWebhook(body, secret, "invalid_signature"))
}

func TestVerifyWebhook_EmptyBody(t *testing.T) {
	assert.False(t, hmacpkg.VerifyWebhook(nil, "secret", "sig"))
	assert.False(t, hmacpkg.VerifyWebhook([]byte{}, "secret", "sig"))
}

func TestVerifyWebhook_EmptySecret(t *testing.T) {
	body := []byte(`{"id": 1}`)
	assert.False(t, hmacpkg.VerifyWebhook(body, "", "sig"))
}

func TestVerifyWebhook_EmptySignature(t *testing.T) {
	body := []byte(`{"id": 1}`)
	assert.False(t, hmacpkg.VerifyWebhook(body, "secret", ""))
}

func TestVerifyWebhook_ModifiedBody(t *testing.T) {
	secret := "test_secret"
	originalBody := []byte(`{"id": 123}`)
	modifiedBody := []byte(`{"id": 124}`)

	sig := hmacpkg.ComputeHMAC(originalBody, secret)

	// Modified body should fail
	assert.False(t, hmacpkg.VerifyWebhook(modifiedBody, secret, sig))
}

func TestVerifyWebhook_RawBodyPreservation(t *testing.T) {
	// This tests that HMAC verification works with the raw body
	// even if the JSON has different formatting
	secret := "my_webhook_secret"

	// Simulate a raw body from Shopify (exact bytes matter)
	rawBody := []byte(`{"id":456,"title":"Test Product"}`)
	sig := hmacpkg.ComputeHMAC(rawBody, secret)

	assert.True(t, hmacpkg.VerifyWebhook(rawBody, secret, sig))

	// Re-serialized body with different whitespace should fail
	reserialized := []byte(`{"id": 456, "title": "Test Product"}`)
	assert.False(t, hmacpkg.VerifyWebhook(reserialized, secret, sig))
}

func TestComputeHMAC_Deterministic(t *testing.T) {
	body := []byte("test message")
	secret := "secret"

	h1 := hmacpkg.ComputeHMAC(body, secret)
	h2 := hmacpkg.ComputeHMAC(body, secret)

	assert.Equal(t, h1, h2)
}
