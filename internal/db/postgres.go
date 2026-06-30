package db

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

func NewPostgres(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	return db, nil
}

// EnsureSchema applies idempotent ALTER TABLE statements that are too small to
// warrant a dedicated migration runner. Each statement must be safe to run on
// every startup.
func EnsureSchema(db *sql.DB) error {
	stmts := []string{
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS vip_exp BIGINT NOT NULL DEFAULT 0`,
		`CREATE TABLE IF NOT EXISTS user_vip_claims (uid BIGINT NOT NULL, level_id INT NOT NULL, claimed_at BIGINT NOT NULL DEFAULT 0, PRIMARY KEY (uid, level_id))`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("apply schema: %w", err)
		}
	}
	return nil
}
