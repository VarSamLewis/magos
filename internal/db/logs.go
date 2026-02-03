package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// LogDB handles logging to SQLite database.
type LogDB struct {
	db *sql.DB
}

// NewLogDB creates a new log database in .magos/logs.db
func NewLogDB(projectRoot string) (*LogDB, error) {
	magosDir := filepath.Join(projectRoot, ".magos")
	if err := os.MkdirAll(magosDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create .magos directory: %w", err)
	}

	dbPath := filepath.Join(magosDir, "logs.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open log database: %w", err)
	}

	if err := createLogsTable(db); err != nil {
		return nil, err
	}

	return &LogDB{db: db}, nil
}

// createLogsTable creates the logs table if it doesn't exist.
func createLogsTable(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		level TEXT NOT NULL,
		datetime TEXT NOT NULL,
		message TEXT NOT NULL,
		file TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_logs_datetime ON logs(datetime);
	CREATE INDEX IF NOT EXISTS idx_logs_level ON logs(level);
	`

	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create logs table: %w", err)
	}

	return nil
}

// Log writes a log entry to the database.
func (l *LogDB) Log(level, message, file string) error {
	datetime := time.Now().Format(time.RFC3339)

	_, err := l.db.Exec(
		"INSERT INTO logs (level, datetime, message, file) VALUES (?, ?, ?, ?)",
		level, datetime, message, file,
	)

	return err
}

// Close closes the database connection.
func (l *LogDB) Close() error {
	if l.db != nil {
		return l.db.Close()
	}
	return nil
}

// SQLiteHandler is a custom slog.Handler that writes to SQLite.
type SQLiteHandler struct {
	logDB *LogDB
	level slog.Level
}

// NewSQLiteHandler creates a new SQLite logger handler.
func NewSQLiteHandler(logDB *LogDB, level slog.Level) *SQLiteHandler {
	return &SQLiteHandler{
		logDB: logDB,
		level: level,
	}
}

// Enabled returns whether the handler is enabled for the given level.
func (h *SQLiteHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

// Handle writes the log record to SQLite.
func (h *SQLiteHandler) Handle(_ context.Context, r slog.Record) error {
	level := r.Level.String()
	message := r.Message

	var file string
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "file" {
			file = a.Value.String()
		}
		return true
	})

	return h.logDB.Log(level, message, file)
}

// WithAttrs returns a new handler with the given attributes.
func (h *SQLiteHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

// WithGroup returns a new handler with the given group.
func (h *SQLiteHandler) WithGroup(name string) slog.Handler {
	return h
}
