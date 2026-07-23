package gendb

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// schemaSQL is the canonical gateway schema, embedded so the binary is
// self-initializing: on startup the gateway applies it (idempotent CREATE TABLE
// IF NOT EXISTS), so a fresh Pi comes up with the right tables without a separate
// migration step. Keep this file in sync with migrations/001_init.sql.
//
//go:embed schema.sql
var schemaSQL string

// Migrate applies the embedded schema statement by statement. It is safe to run
// on every boot. The DSN uses multiStatements=false, so statements are split and
// executed individually (the schema contains no ';' inside a statement).
func Migrate() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stmts := splitStatements(schemaSQL)
	for _, s := range stmts {
		if _, err := DB().ExecContext(ctx, s); err != nil {
			return fmt.Errorf("gendb: migrate statement failed: %w\n--- statement ---\n%s", err, s)
		}
	}
	slog.Info("schema applied", "statements", len(stmts))
	return nil
}

// splitStatements strips line comments and splits on ';'. The schema is written
// so no ';' appears inside a string or identifier.
func splitStatements(sql string) []string {
	var b strings.Builder
	for _, line := range strings.Split(sql, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") {
			continue // whole-line comment
		}
		// Strip a trailing inline "-- ..." comment (the schema never puts '--'
		// inside a quoted string).
		if i := strings.Index(line, "--"); i >= 0 {
			line = line[:i]
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	var out []string
	for _, part := range strings.Split(b.String(), ";") {
		if strings.TrimSpace(part) != "" {
			out = append(out, strings.TrimSpace(part))
		}
	}
	return out
}
