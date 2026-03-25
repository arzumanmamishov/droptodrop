package unit

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/droptodrop/droptodrop/pkg/idempotency"
)

func TestGenerateKey_Deterministic(t *testing.T) {
	key1 := idempotency.GenerateKey("route_order", "shop_123", "order_456")
	key2 := idempotency.GenerateKey("route_order", "shop_123", "order_456")

	assert.Equal(t, key1, key2, "same inputs should produce same key")
}

func TestGenerateKey_DifferentInputs(t *testing.T) {
	key1 := idempotency.GenerateKey("route_order", "shop_123", "order_456")
	key2 := idempotency.GenerateKey("route_order", "shop_123", "order_789")

	assert.NotEqual(t, key1, key2, "different inputs should produce different keys")
}

func TestGenerateKey_OrderMatters(t *testing.T) {
	key1 := idempotency.GenerateKey("a", "b", "c")
	key2 := idempotency.GenerateKey("c", "b", "a")

	assert.NotEqual(t, key1, key2, "argument order should matter")
}
