package unit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/droptodrop/droptodrop/pkg/sessiontoken"
)

const testAPIKey = "test_api_key_123"
const testAPISecret = "test_api_secret_456"

func validClaims() sessiontoken.Claims {
	now := time.Now()
	return sessiontoken.Claims{
		ISS:  "https://test-store.myshopify.com/admin",
		Dest: "https://test-store.myshopify.com",
		Aud:  testAPIKey,
		Sub:  "42",
		Exp:  now.Add(5 * time.Minute).Unix(),
		Nbf:  now.Add(-1 * time.Second).Unix(),
		Iat:  now.Unix(),
		Jti:  "unique-token-id-123",
		Sid:  "session-id-abc",
	}
}

func testConfig() sessiontoken.VerifyConfig {
	return sessiontoken.VerifyConfig{
		APIKey:    testAPIKey,
		APISecret: testAPISecret,
		ClockSkew: 10 * time.Second,
	}
}

func TestVerify_ValidToken(t *testing.T) {
	claims := validClaims()
	token := sessiontoken.BuildTestToken(claims, testAPISecret)

	result, err := sessiontoken.Verify(token, testConfig())
	require.NoError(t, err)

	assert.Equal(t, claims.ISS, result.ISS)
	assert.Equal(t, claims.Dest, result.Dest)
	assert.Equal(t, claims.Aud, result.Aud)
	assert.Equal(t, claims.Sub, result.Sub)
	assert.Equal(t, claims.Jti, result.Jti)
	assert.Equal(t, claims.Sid, result.Sid)
}

func TestVerify_ShopDomain(t *testing.T) {
	claims := validClaims()
	token := sessiontoken.BuildTestToken(claims, testAPISecret)

	result, err := sessiontoken.Verify(token, testConfig())
	require.NoError(t, err)

	assert.Equal(t, "test-store.myshopify.com", result.ShopDomain())
}

func TestVerify_InvalidSignature(t *testing.T) {
	claims := validClaims()
	token := sessiontoken.BuildTestToken(claims, "wrong_secret")

	_, err := sessiontoken.Verify(token, testConfig())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "signature verification failed")
}

func TestVerify_ExpiredToken(t *testing.T) {
	claims := validClaims()
	claims.Exp = time.Now().Add(-1 * time.Minute).Unix()
	token := sessiontoken.BuildTestToken(claims, testAPISecret)

	_, err := sessiontoken.Verify(token, testConfig())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token expired")
}

func TestVerify_ExpiredTokenWithinClockSkew(t *testing.T) {
	claims := validClaims()
	// Expired 5 seconds ago, but clock skew is 10 seconds → should pass
	claims.Exp = time.Now().Add(-5 * time.Second).Unix()
	token := sessiontoken.BuildTestToken(claims, testAPISecret)

	result, err := sessiontoken.Verify(token, testConfig())
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestVerify_NotYetValid(t *testing.T) {
	claims := validClaims()
	claims.Nbf = time.Now().Add(1 * time.Minute).Unix()
	token := sessiontoken.BuildTestToken(claims, testAPISecret)

	_, err := sessiontoken.Verify(token, testConfig())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not yet valid")
}

func TestVerify_WrongAudience(t *testing.T) {
	claims := validClaims()
	claims.Aud = "wrong_api_key"
	token := sessiontoken.BuildTestToken(claims, testAPISecret)

	_, err := sessiontoken.Verify(token, testConfig())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "audience mismatch")
}

func TestVerify_InvalidDest(t *testing.T) {
	claims := validClaims()
	claims.Dest = "https://evil.com"
	token := sessiontoken.BuildTestToken(claims, testAPISecret)

	_, err := sessiontoken.Verify(token, testConfig())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid dest claim")
}

func TestVerify_MalformedToken_TwoParts(t *testing.T) {
	_, err := sessiontoken.Verify("header.payload", testConfig())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected 3 parts")
}

func TestVerify_MalformedToken_FourParts(t *testing.T) {
	_, err := sessiontoken.Verify("a.b.c.d", testConfig())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected 3 parts")
}

func TestVerify_MalformedToken_EmptyString(t *testing.T) {
	_, err := sessiontoken.Verify("", testConfig())
	require.Error(t, err)
}

func TestVerify_MalformedToken_InvalidBase64(t *testing.T) {
	_, err := sessiontoken.Verify("not!valid.base!64.sig!!!", testConfig())
	require.Error(t, err)
}

func TestVerify_TamperedPayload(t *testing.T) {
	claims := validClaims()
	token := sessiontoken.BuildTestToken(claims, testAPISecret)

	// Tamper with the payload (swap a character)
	parts := splitToken(token)
	// Modify the payload portion
	tampered := parts[0] + "." + parts[1] + "x" + "." + parts[2]

	_, err := sessiontoken.Verify(tampered, testConfig())
	require.Error(t, err)
}

func TestVerify_DefaultClockSkew(t *testing.T) {
	claims := validClaims()
	token := sessiontoken.BuildTestToken(claims, testAPISecret)

	// Config with zero clock skew should default to 10s
	cfg := sessiontoken.VerifyConfig{
		APIKey:    testAPIKey,
		APISecret: testAPISecret,
	}

	result, err := sessiontoken.Verify(token, cfg)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestShopDomain_VariousFormats(t *testing.T) {
	tests := []struct {
		dest     string
		expected string
	}{
		{"https://my-store.myshopify.com", "my-store.myshopify.com"},
		{"https://my-store.myshopify.com/", "my-store.myshopify.com"},
		{"http://my-store.myshopify.com", "my-store.myshopify.com"},
	}

	for _, tt := range tests {
		c := &sessiontoken.Claims{Dest: tt.dest}
		assert.Equal(t, tt.expected, c.ShopDomain(), "dest=%s", tt.dest)
	}
}

func TestBuildTestToken_Roundtrip(t *testing.T) {
	claims := validClaims()
	token := sessiontoken.BuildTestToken(claims, testAPISecret)

	// Should be three dot-separated parts
	parts := splitToken(token)
	assert.Len(t, parts, 3)

	// Each part should be non-empty
	for i, p := range parts {
		assert.NotEmpty(t, p, "part %d should not be empty", i)
	}

	// Should verify successfully
	result, err := sessiontoken.Verify(token, testConfig())
	require.NoError(t, err)
	assert.Equal(t, claims.Sub, result.Sub)
}

func splitToken(token string) []string {
	parts := make([]string, 0)
	current := ""
	for _, c := range token {
		if c == '.' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	parts = append(parts, current)
	return parts
}
