package shopify

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// OAuthConfig holds Shopify OAuth configuration.
type OAuthConfig struct {
	APIKey      string
	APISecret   string
	Scopes      string
	RedirectURI string
}

// TokenResponse is the response from Shopify's token exchange.
type TokenResponse struct {
	AccessToken    string `json:"access_token"`
	Scope          string `json:"scope"`
	AssociatedUser *struct {
		ID    int64  `json:"id"`
		Email string `json:"email"`
	} `json:"associated_user,omitempty"`
}

// BuildAuthURL constructs the Shopify OAuth authorization URL.
func BuildAuthURL(shop string, cfg OAuthConfig, nonce string) string {
	params := url.Values{
		"client_id":    {cfg.APIKey},
		"scope":        {cfg.Scopes},
		"redirect_uri": {cfg.RedirectURI},
		"state":        {nonce},
	}
	return fmt.Sprintf("https://%s/admin/oauth/authorize?%s", shop, params.Encode())
}

// ValidateCallback verifies the OAuth callback parameters.
func ValidateCallback(query url.Values, apiSecret string) bool {
	sig := query.Get("hmac")
	if sig == "" {
		return false
	}

	// Build the message from sorted query params, excluding hmac
	params := make([]string, 0)
	for key, vals := range query {
		if key == "hmac" {
			continue
		}
		for _, val := range vals {
			params = append(params, fmt.Sprintf("%s=%s", key, val))
		}
	}
	sort.Strings(params)
	message := strings.Join(params, "&")

	mac := hmac.New(sha256.New, []byte(apiSecret))
	mac.Write([]byte(message))
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(sig))
}

// ExchangeToken exchanges the authorization code for an access token.
func ExchangeToken(ctx context.Context, shop, code string, cfg OAuthConfig) (*TokenResponse, error) {
	body := url.Values{
		"client_id":     {cfg.APIKey},
		"client_secret": {cfg.APISecret},
		"code":          {code},
	}

	tokenURL := fmt.Sprintf("https://%s/admin/oauth/access_token", shop)

	httpClient := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(body.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token exchange request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return nil, fmt.Errorf("unmarshal token response: %w", err)
	}

	return &tokenResp, nil
}

// ValidateShopDomain validates that a shop domain looks correct.
func ValidateShopDomain(shop string) bool {
	if shop == "" {
		return false
	}
	// Must match: store-name.myshopify.com
	if !strings.HasSuffix(shop, ".myshopify.com") {
		return false
	}
	name := strings.TrimSuffix(shop, ".myshopify.com")
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "..") {
		return false
	}
	return true
}
