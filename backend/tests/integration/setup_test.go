package integration

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

var testDB *pgxpool.Pool

func TestMain(m *testing.M) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		// Skip integration tests if no database is available
		os.Exit(0)
	}

	var err error
	testDB, err = pgxpool.New(context.Background(), dbURL)
	if err != nil {
		panic("failed to connect to test database: " + err.Error())
	}
	defer testDB.Close()

	os.Exit(m.Run())
}

func cleanTable(t *testing.T, table string) {
	t.Helper()
	_, err := testDB.Exec(context.Background(), "DELETE FROM "+table)
	if err != nil {
		t.Fatalf("failed to clean table %s: %v", table, err)
	}
}
