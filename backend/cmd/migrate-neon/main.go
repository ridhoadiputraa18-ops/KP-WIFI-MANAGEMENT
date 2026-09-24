package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

type Table struct {
	Name    string
	Columns []string
}

var tables = []Table{
	{Name: "roles", Columns: []string{"id", "name"}},
	{Name: "users", Columns: []string{"id", "role_id", "username", "password_hash", "full_name", "email", "status", "created_at", "updated_at"}},
	{Name: "members", Columns: []string{"id", "user_id", "member_code", "access_start", "access_end", "status", "created_at"}},
	{Name: "guests", Columns: []string{"id", "name", "access_start", "access_end", "status", "created_at"}},
	{Name: "devices", Columns: []string{"id", "name", "device_type", "vendor", "model", "ip_address", "mac_address", "api_endpoint", "status", "last_seen", "created_at"}},
	{Name: "sessions", Columns: []string{"id", "user_id", "guest_id", "device_id", "client_ip", "client_mac", "started_at", "ended_at", "status"}},
	{Name: "usage_logs", Columns: []string{"id", "session_id", "upload_bytes", "download_bytes", "total_bytes", "recorded_at"}},
	{Name: "wifi_access", Columns: []string{"id", "user_id", "guest_id", "ssid", "access_type", "start_at", "end_at", "status"}},
	{Name: "wifi_configs", Columns: []string{"id", "ssid", "network_type", "password_encrypted", "guest_duration_minutes", "status", "created_at", "updated_at"}},
	{Name: "device_metrics", Columns: []string{"id", "device_id", "cpu_usage", "memory_usage", "uptime_seconds", "recorded_at"}},
	{Name: "audit_logs", Columns: []string{"id", "user_id", "action", "target_type", "target_id", "description", "ip_address", "created_at"}},
}

func placeholders(n int) string {
	p := make([]string, n)
	for i := range p {
		p[i] = fmt.Sprintf("$%d", i+1)
	}
	return strings.Join(p, ", ")
}

func migrateTable(sqliteDB, pgDB *sql.DB, table Table) error {
	selectSQL := fmt.Sprintf(
		"SELECT %s FROM %s ORDER BY id",
		strings.Join(table.Columns, ", "),
		table.Name,
	)

	rows, err := sqliteDB.Query(selectSQL)
	if err != nil {
		return fmt.Errorf("SQLite %s: %w", table.Name, err)
	}
	defer rows.Close()

	insertSQL := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (id) DO NOTHING",
		table.Name,
		strings.Join(table.Columns, ", "),
		placeholders(len(table.Columns)),
	)

	count := 0

	for rows.Next() {
		values := make([]any, len(table.Columns))
		dest := make([]any, len(table.Columns))

		for i := range values {
			dest[i] = &values[i]
		}

		if err := rows.Scan(dest...); err != nil {
			return fmt.Errorf("scan %s: %w", table.Name, err)
		}

		if _, err := pgDB.Exec(insertSQL, values...); err != nil {
			return fmt.Errorf("insert %s id=%v: %w", table.Name, values[0], err)
		}

		count++
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows %s: %w", table.Name, err)
	}

	fmt.Printf("  %-15s %d row(s)\n", table.Name, count)
	return nil
}

func resetSequence(pgDB *sql.DB, table string) error {
	query := fmt.Sprintf(`
		SELECT setval(
			pg_get_serial_sequence('%s', 'id'),
			COALESCE(MAX(id), 1),
			MAX(id) IS NOT NULL
		)
		FROM %s
	`, table, table)

	if _, err := pgDB.Exec(query); err != nil {
		return fmt.Errorf("reset sequence %s: %w", table, err)
	}

	return nil
}

func main() {
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))

	if databaseURL == "" {
		log.Fatal("DATABASE_URL belum di-set.")
	}

	sqliteDB, err := sql.Open("sqlite", "../database.db")
	if err != nil {
		log.Fatal("Gagal membuka SQLite:", err)
	}
	defer sqliteDB.Close()

	if err := sqliteDB.Ping(); err != nil {
		log.Fatal("SQLite tidak bisa diakses:", err)
	}

	fmt.Println("SQLite lokal: OK")

	pgDB, err := sql.Open("pgx", databaseURL)
	if err != nil {
		log.Fatal("Gagal membuka PostgreSQL:", err)
	}
	defer pgDB.Close()

	if err := pgDB.Ping(); err != nil {
		log.Fatal("Neon PostgreSQL tidak bisa diakses:", err)
	}

	fmt.Println("Neon PostgreSQL: OK")
	fmt.Println()
	fmt.Println("Mulai migrasi data...")
	fmt.Println()

	for _, table := range tables {
		if err := migrateTable(sqliteDB, pgDB, table); err != nil {
			log.Fatal(err)
		}
	}

	fmt.Println()
	fmt.Println("Reset sequence PostgreSQL...")

	for _, table := range tables {
		if err := resetSequence(pgDB, table.Name); err != nil {
			log.Fatal(err)
		}
	}

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println(" MIGRASI SQLITE -> NEON BERHASIL")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("auth_sessions sengaja tidak dimigrasikan")
	fmt.Println("karena berisi session login/token lama.")
}
