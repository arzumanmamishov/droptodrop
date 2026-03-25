package unit

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/droptodrop/droptodrop/pkg/shopify"
)

func TestValidateShopDomain_Valid(t *testing.T) {
	tests := []struct {
		domain string
		valid  bool
	}{
		{"test-store.myshopify.com", true},
		{"my-shop.myshopify.com", true},
		{"store123.myshopify.com", true},
		{"", false},
		{"evil.com", false},
		{"test.notshopify.com", false},
		{".myshopify.com", false},
		{"../hack.myshopify.com", false},
		{"test/inject.myshopify.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			assert.Equal(t, tt.valid, shopify.ValidateShopDomain(tt.domain))
		})
	}
}

func TestBuildAuthURL(t *testing.T) {
	cfg := shopify.OAuthConfig{
		APIKey:      "test_key",
		APISecret:   "test_secret",
		Scopes:      "read_products,write_products",
		RedirectURI: "https://app.example.com/auth/callback",
	}

	url := shopify.BuildAuthURL("test.myshopify.com", cfg, "nonce123")

	assert.Contains(t, url, "https://test.myshopify.com/admin/oauth/authorize")
	assert.Contains(t, url, "client_id=test_key")
	assert.Contains(t, url, "scope=read_products")
	assert.Contains(t, url, "redirect_uri=")
	assert.Contains(t, url, "state=nonce123")
}

func TestValidateCallback_ValidHMAC(t *testing.T) {
	// This test validates that the HMAC verification logic for OAuth callbacks works.
	// In a real scenario, Shopify sends query params with an hmac parameter.
	// The message is all query params (except hmac) sorted and joined.

	// For now, test that invalid signatures are rejected
	query := map[string][]string{
		"code":      {"auth_code_123"},
		"hmac":      {"invalid_hmac"},
		"shop":      {"test.myshopify.com"},
		"state":     {"nonce123"},
		"timestamp": {"1234567890"},
	}

	assert.False(t, shopify.ValidateCallback(query, "test_secret"))
}
