package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShopCreate_Integration(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}

	cleanTable(t, "audit_logs")
	cleanTable(t, "shop_sessions")
	cleanTable(t, "app_settings")
	cleanTable(t, "app_installations")
	cleanTable(t, "supplier_profiles")
	cleanTable(t, "reseller_profiles")
	cleanTable(t, "shops")

	ctx := context.Background()

	// Create a shop
	var shopID string
	err := testDB.QueryRow(ctx, `
		INSERT INTO shops (shopify_domain, name, role, status)
		VALUES ('integration-test.myshopify.com', 'Integration Test', 'unset', 'active')
		RETURNING id
	`).Scan(&shopID)
	require.NoError(t, err)
	assert.NotEmpty(t, shopID)

	// Verify it exists
	var domain, role string
	err = testDB.QueryRow(ctx, `SELECT shopify_domain, role FROM shops WHERE id = $1`, shopID).Scan(&domain, &role)
	require.NoError(t, err)
	assert.Equal(t, "integration-test.myshopify.com", domain)
	assert.Equal(t, "unset", role)

	// Set role to supplier
	_, err = testDB.Exec(ctx, `UPDATE shops SET role = 'supplier' WHERE id = $1`, shopID)
	require.NoError(t, err)

	// Create supplier profile
	_, err = testDB.Exec(ctx, `INSERT INTO supplier_profiles (shop_id) VALUES ($1)`, shopID)
	require.NoError(t, err)

	// Verify profile
	var isEnabled bool
	err = testDB.QueryRow(ctx, `SELECT is_enabled FROM supplier_profiles WHERE shop_id = $1`, shopID).Scan(&isEnabled)
	require.NoError(t, err)
	assert.False(t, isEnabled)
}

func TestUninstallFlow_Integration(t *testing.T) {
	if testDB == nil {
		t.Skip("no database connection")
	}

	cleanTable(t, "audit_logs")
	cleanTable(t, "shop_sessions")
	cleanTable(t, "app_settings")
	cleanTable(t, "app_installations")
	cleanTable(t, "supplier_profiles")
	cleanTable(t, "reseller_profiles")
	cleanTable(t, "shops")

	ctx := context.Background()

	// Create shop and installation
	var shopID string
	err := testDB.QueryRow(ctx, `
		INSERT INTO shops (shopify_domain, name, role, status)
		VALUES ('uninstall-test.myshopify.com', 'Uninstall Test', 'supplier', 'active')
		RETURNING id
	`).Scan(&shopID)
	require.NoError(t, err)

	_, err = testDB.Exec(ctx, `
		INSERT INTO app_installations (shop_id, access_token, scopes, is_active)
		VALUES ($1, 'token', 'scopes', TRUE)
	`, shopID)
	require.NoError(t, err)

	_, err = testDB.Exec(ctx, `
		INSERT INTO shop_sessions (shop_id, session_token, expires_at)
		VALUES ($1, 'session_123', NOW() + INTERVAL '1 day')
	`, shopID)
	require.NoError(t, err)

	// Simulate uninstall
	_, err = testDB.Exec(ctx, `UPDATE shops SET status = 'uninstalled' WHERE id = $1`, shopID)
	require.NoError(t, err)
	_, err = testDB.Exec(ctx, `UPDATE app_installations SET is_active = FALSE, uninstalled_at = NOW() WHERE shop_id = $1`, shopID)
	require.NoError(t, err)
	_, err = testDB.Exec(ctx, `DELETE FROM shop_sessions WHERE shop_id = $1`, shopID)
	require.NoError(t, err)

	// Verify state
	var status string
	err = testDB.QueryRow(ctx, `SELECT status FROM shops WHERE id = $1`, shopID).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "uninstalled", status)

	var isActive bool
	err = testDB.QueryRow(ctx, `SELECT is_active FROM app_installations WHERE shop_id = $1`, shopID).Scan(&isActive)
	require.NoError(t, err)
	assert.False(t, isActive)

	// Sessions should be gone
	var sessionCount int
	err = testDB.QueryRow(ctx, `SELECT COUNT(*) FROM shop_sessions WHERE shop_id = $1`, shopID).Scan(&sessionCount)
	require.NoError(t, err)
	assert.Equal(t, 0, sessionCount)
}
