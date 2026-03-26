package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect error: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(ctx)

	if len(os.Args) > 1 && os.Args[1] == "list" {
		rows, err := conn.Query(ctx, "SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename")
		if err != nil {
			fmt.Fprintf(os.Stderr, "query error: %v\n", err)
			os.Exit(1)
		}
		defer rows.Close()
		fmt.Println("Tables:")
		for rows.Next() {
			var t string
			rows.Scan(&t)
			fmt.Printf("  - %s\n", t)
		}
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "list-installs" {
		rows, err := conn.Query(ctx, "SELECT id, shop_id, is_active, scopes FROM app_installations ORDER BY created_at")
		if err != nil {
			fmt.Fprintf(os.Stderr, "query error: %v\n", err)
			os.Exit(1)
		}
		defer rows.Close()
		fmt.Println("Installations:")
		for rows.Next() {
			var id, shopID, scopes string
			var active bool
			rows.Scan(&id, &shopID, &active, &scopes)
			fmt.Printf("  - %s | shop=%s | active=%v | scopes=%s\n", id, shopID, active, scopes)
		}
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "list-shops" {
		rows, err := conn.Query(ctx, "SELECT id, shopify_domain, role, status FROM shops ORDER BY created_at")
		if err != nil {
			fmt.Fprintf(os.Stderr, "query error: %v\n", err)
			os.Exit(1)
		}
		defer rows.Close()
		fmt.Println("Shops:")
		for rows.Next() {
			var id, domain, role, status string
			rows.Scan(&id, &domain, &role, &status)
			fmt.Printf("  - %s | %s | role=%s | status=%s\n", id, domain, role, status)
		}
		return
	}

	sql, err := os.ReadFile("migrations/000001_init_schema.up.sql")
	if err != nil {
		fmt.Fprintf(os.Stderr, "read migration: %v\n", err)
		os.Exit(1)
	}

	_, err = conn.Exec(ctx, string(sql))
	if err != nil {
		fmt.Fprintf(os.Stderr, "migration error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Migration completed successfully!")
}
