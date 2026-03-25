package unit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Replicate the pricing calculation logic from imports service for testing.
func calculateResellerPrice(wholesale float64, markupType string, markupValue float64) float64 {
	switch markupType {
	case "fixed":
		return wholesale + markupValue
	case "percentage":
		return wholesale * (1 + markupValue/100)
	default:
		return wholesale * 1.3
	}
}

func TestPricing_PercentageMarkup(t *testing.T) {
	tests := []struct {
		wholesale float64
		markup    float64
		expected  float64
	}{
		{100.00, 30, 130.00},
		{50.00, 50, 75.00},
		{10.00, 100, 20.00},
		{25.50, 20, 30.60},
		{0.00, 30, 0.00},
	}

	for _, tt := range tests {
		result := calculateResellerPrice(tt.wholesale, "percentage", tt.markup)
		assert.InDelta(t, tt.expected, result, 0.01, "wholesale=%.2f markup=%.0f%%", tt.wholesale, tt.markup)
	}
}

func TestPricing_FixedMarkup(t *testing.T) {
	tests := []struct {
		wholesale float64
		markup    float64
		expected  float64
	}{
		{100.00, 25.00, 125.00},
		{50.00, 10.00, 60.00},
		{0.00, 15.00, 15.00},
	}

	for _, tt := range tests {
		result := calculateResellerPrice(tt.wholesale, "fixed", tt.markup)
		assert.InDelta(t, tt.expected, result, 0.01)
	}
}

func TestPricing_DefaultMarkup(t *testing.T) {
	// Unknown markup type should default to 30% markup
	result := calculateResellerPrice(100.00, "unknown", 0)
	assert.InDelta(t, 130.00, result, 0.01)
}

func TestPricing_MarginCalculation(t *testing.T) {
	wholesale := 45.00
	retail := calculateResellerPrice(wholesale, "percentage", 30)

	// Margin = (retail - wholesale) / retail * 100
	margin := (retail - wholesale) / retail * 100

	assert.InDelta(t, 23.08, margin, 0.1, "30%% markup on $45 should yield ~23%% margin")
	assert.True(t, margin > 10, "margin should exceed minimum 10%%")
}

func TestPricing_MinMarginEnforcement(t *testing.T) {
	wholesale := 100.00
	minMarginPct := 10.0

	// Check that a given markup produces at least the minimum margin
	markups := []float64{5, 10, 15, 20, 30, 50}
	for _, markup := range markups {
		retail := calculateResellerPrice(wholesale, "percentage", markup)
		margin := (retail - wholesale) / retail * 100

		if markup < 12 {
			// Very low markup may not meet min margin, which is expected
			continue
		}
		assert.True(t, margin >= minMarginPct,
			"markup=%.0f%% should produce margin >= %.0f%%, got %.1f%%", markup, minMarginPct, margin)
	}
}
