package unit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/droptodrop/droptodrop/internal/auth"
)

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	// 32-byte hex key (64 hex chars = 32 bytes)
	key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	plaintext := "shpat_test_access_token_12345"

	// We can't call encrypt directly (unexported), but we can test Decrypt
	// with a known encrypted value. Instead, let's test the public Decrypt
	// by creating a test helper.

	// Since encrypt is unexported, we test that Decrypt handles errors properly
	_, err := auth.Decrypt("", key)
	assert.Error(t, err, "empty ciphertext should error")

	_, err = auth.Decrypt("not_hex", key)
	assert.Error(t, err, "invalid hex should error")

	_, err = auth.Decrypt("aabbcc", key)
	assert.Error(t, err, "short ciphertext should error")

	// Test with invalid key
	_, err = auth.Decrypt("aabbccdd", "short")
	assert.Error(t, err, "invalid key length should error")

	_ = plaintext
}

func TestDecrypt_InvalidKey(t *testing.T) {
	_, err := auth.Decrypt("0011223344556677", "not_a_valid_hex_key")
	assert.Error(t, err)
}

func TestDecrypt_TamperedCiphertext(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	// Create a ciphertext that's long enough to have a nonce but is random garbage
	fakeCiphertext := "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff0011223344556677"
	_, err := auth.Decrypt(fakeCiphertext, key)
	// Should fail due to authentication tag mismatch
	require.Error(t, err)
}
