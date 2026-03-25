package unit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/droptodrop/droptodrop/pkg/shopify"
)

func TestParseGID_Product(t *testing.T) {
	id, err := shopify.ParseGID("gid://shopify/Product/123456789")
	require.NoError(t, err)
	assert.Equal(t, int64(123456789), id)
}

func TestParseGID_Variant(t *testing.T) {
	id, err := shopify.ParseGID("gid://shopify/ProductVariant/987654321")
	require.NoError(t, err)
	assert.Equal(t, int64(987654321), id)
}

func TestParseGID_Fulfillment(t *testing.T) {
	id, err := shopify.ParseGID("gid://shopify/Fulfillment/111222333")
	require.NoError(t, err)
	assert.Equal(t, int64(111222333), id)
}

func TestParseGID_Order(t *testing.T) {
	id, err := shopify.ParseGID("gid://shopify/Order/444555666")
	require.NoError(t, err)
	assert.Equal(t, int64(444555666), id)
}

func TestParseGID_LargeID(t *testing.T) {
	id, err := shopify.ParseGID("gid://shopify/Product/9999999999999")
	require.NoError(t, err)
	assert.Equal(t, int64(9999999999999), id)
}

func TestParseGID_Invalid_NoSlash(t *testing.T) {
	_, err := shopify.ParseGID("invalid")
	assert.Error(t, err)
}

func TestParseGID_Invalid_Empty(t *testing.T) {
	_, err := shopify.ParseGID("")
	assert.Error(t, err)
}

func TestParseGID_Invalid_NonNumeric(t *testing.T) {
	_, err := shopify.ParseGID("gid://shopify/Product/abc")
	assert.Error(t, err)
}
