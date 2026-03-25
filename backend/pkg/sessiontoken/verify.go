// Package sessiontoken implements Shopify App Bridge session token (JWT) verification.
//
// Shopify App Bridge sends session tokens as JWTs signed with HS256 using the
// app's API secret. The token contains claims identifying the shop, user, and
// permissions.
//
// Reference: https://shopify.dev/docs/apps/auth/session-tokens
package sessiontoken

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Claims represents the decoded claims from a Shopify session token JWT.
type Claims struct {
	// ISS is the shop's admin URL: https://{shop}.myshopify.com/admin
	ISS string `json:"iss"`
	// Dest is the shop URL: https://{shop}.myshopify.com
	Dest string `json:"dest"`
	// Aud is the app's API key
	Aud string `json:"aud"`
	// Sub is the user ID (shop staff member who initiated the request)
	Sub string `json:"sub"`
	// Exp is the token expiry (UNIX timestamp)
	Exp int64 `json:"exp"`
	// Nbf is the "not before" time (UNIX timestamp)
	Nbf int64 `json:"nbf"`
	// Iat is the "issued at" time (UNIX timestamp)
	Iat int64 `json:"iat"`
	// Jti is the unique token identifier
	Jti string `json:"jti"`
	// Sid is the session ID
	Sid string `json:"sid"`
}

// ShopDomain extracts the myshopify.com domain from the ISS or Dest claim.
// Returns e.g. "my-store.myshopify.com"
func (c *Claims) ShopDomain() string {
	// Dest is "https://{shop}.myshopify.com"
	domain := c.Dest
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimSuffix(domain, "/")
	return domain
}

// VerifyConfig holds the configuration needed to verify session tokens.
type VerifyConfig struct {
	// APIKey is the Shopify app's API key (used to verify the 'aud' claim)
	APIKey string
	// APISecret is the Shopify app's API secret (used to verify the HS256 signature)
	APISecret string
	// ClockSkew is the maximum allowed clock skew for token expiry checks.
	// Defaults to 10 seconds if zero.
	ClockSkew time.Duration
}

// Verify decodes and verifies a Shopify App Bridge session token.
// It checks:
//   - JWT structure (3 parts, base64url-encoded)
//   - HS256 signature against APISecret
//   - Algorithm is HS256
//   - 'aud' matches APIKey
//   - Token is not expired (with clock skew tolerance)
//   - Token 'nbf' is in the past (with clock skew tolerance)
//
// Returns the decoded claims on success, or an error describing the failure.
func Verify(tokenString string, cfg VerifyConfig) (*Claims, error) {
	if cfg.ClockSkew == 0 {
		cfg.ClockSkew = 10 * time.Second
	}

	// Split into header.payload.signature
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token: expected 3 parts, got %d", len(parts))
	}

	headerB64, payloadB64, signatureB64 := parts[0], parts[1], parts[2]

	// Verify signature first (before trusting any claims)
	signingInput := headerB64 + "." + payloadB64
	if err := verifyHS256(signingInput, signatureB64, cfg.APISecret); err != nil {
		return nil, fmt.Errorf("signature verification failed: %w", err)
	}

	// Decode and verify header
	headerBytes, err := base64URLDecode(headerB64)
	if err != nil {
		return nil, fmt.Errorf("decode header: %w", err)
	}

	var header struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("parse header: %w", err)
	}
	if header.Alg != "HS256" {
		return nil, fmt.Errorf("unsupported algorithm: %s (expected HS256)", header.Alg)
	}

	// Decode payload
	payloadBytes, err := base64URLDecode(payloadB64)
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}

	var claims Claims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, fmt.Errorf("parse claims: %w", err)
	}

	// Verify audience matches our API key
	if claims.Aud != cfg.APIKey {
		return nil, fmt.Errorf("audience mismatch: got %q, expected %q", claims.Aud, cfg.APIKey)
	}

	// Verify expiry
	now := time.Now()
	expTime := time.Unix(claims.Exp, 0)
	if now.After(expTime.Add(cfg.ClockSkew)) {
		return nil, fmt.Errorf("token expired at %s (current time: %s)", expTime, now)
	}

	// Verify nbf (not before)
	if claims.Nbf > 0 {
		nbfTime := time.Unix(claims.Nbf, 0)
		if now.Before(nbfTime.Add(-cfg.ClockSkew)) {
			return nil, fmt.Errorf("token not yet valid: nbf=%s (current time: %s)", nbfTime, now)
		}
	}

	// Verify dest contains .myshopify.com
	if !strings.Contains(claims.Dest, ".myshopify.com") {
		return nil, fmt.Errorf("invalid dest claim: %q (expected *.myshopify.com)", claims.Dest)
	}

	return &claims, nil
}

// verifyHS256 verifies the HMAC-SHA256 signature of a JWT.
func verifyHS256(signingInput, signatureB64, secret string) error {
	// Compute expected signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	expectedSig := mac.Sum(nil)

	// Decode the provided signature
	actualSig, err := base64URLDecode(signatureB64)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}

	// Constant-time comparison
	if !hmac.Equal(expectedSig, actualSig) {
		return fmt.Errorf("signature mismatch")
	}

	return nil
}

// base64URLDecode decodes a base64url-encoded string (without padding).
func base64URLDecode(s string) ([]byte, error) {
	// Add padding if needed
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	return base64.URLEncoding.DecodeString(s)
}

// BuildTestToken creates a signed JWT for testing purposes.
// NOT for production use.
func BuildTestToken(claims Claims, secret string) string {
	header := `{"alg":"HS256","typ":"JWT"}`
	headerB64 := base64.RawURLEncoding.EncodeToString([]byte(header))

	payload, _ := json.Marshal(claims)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)

	signingInput := headerB64 + "." + payloadB64
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	sig := mac.Sum(nil)
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)

	return signingInput + "." + sigB64
}
