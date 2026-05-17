package mysql

import (
	"context"
	"database/sql"
	"embed"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

//go:embed migration.sql
var migrations embed.FS

func Open(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func ApplyMigrations(ctx context.Context, db *sql.DB) error {
	content, err := migrations.ReadFile("migration.sql")
	if err != nil {
		return err
	}
	for _, stmt := range strings.Split(string(content), ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}
