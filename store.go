package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	_ "modernc.org/sqlite"
)

// Store is the single layer between the application and the database.
// All reads and writes go through WriteTx or ReadTx - they handle
// locking, transactions, and rollback automatically.
type Store struct {
	db *sql.DB
	mu sync.RWMutex
}

func OpenStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".butler")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return OpenStoreAt(filepath.Join(dir, "butler.db"))
}

func OpenStoreAt(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// One connection. SQLite only supports one writer at a time -
	// a pool of connections would just fight over the write lock.
	db.SetMaxOpenConns(1)

	pragmas := []string{
		// WAL = Write-Ahead Log. Writes go to a separate log file first,
		// so a reader doesn't see a half-finished write - it sees the
		// last fully committed state. Without WAL, a write locks the
		// entire database file and readers get "database is locked".
		"PRAGMA journal_mode=WAL",

		// If another process is mid-write, wait up to 5 seconds for it
		// to finish instead of failing immediately with SQLITE_BUSY.
		"PRAGMA busy_timeout=5000",

		// With WAL, NORMAL is safe. Fsync happens on checkpoint, not
		// every commit - faster writes, same crash safety guarantees.
		"PRAGMA synchronous=NORMAL",

		// Enforce foreign key constraints. Off by default in SQLite.
		// Needed when rules reference tasks, etc.
		"PRAGMA foreign_keys=ON",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("%s: %w", p, err)
		}
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

// migrate creates tables. Add new tables here as the app grows.
func migrate(db *sql.DB) error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			parent_id INTEGER REFERENCES tasks(id),
			position INTEGER,
			parallel INTEGER DEFAULT 0,
			status TEXT DEFAULT 'not_started',
			description TEXT DEFAULT '',
			verification TEXT DEFAULT '',
			verify_status TEXT DEFAULT '',
			deadline DATETIME,
			recur TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			status_changed_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS task_blockers (
			task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
			blocker_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
			PRIMARY KEY (task_id, blocker_id)
		)`,
		`CREATE TABLE IF NOT EXISTS rules (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			seq INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS tags (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			ruletag INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS task_tags (
			task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
			tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
			PRIMARY KEY (task_id, tag_id)
		)`,
		`CREATE TABLE IF NOT EXISTS rule_tags (
			rule_id INTEGER NOT NULL REFERENCES rules(id) ON DELETE CASCADE,
			tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
			PRIMARY KEY (rule_id, tag_id)
		)`,
		`CREATE TABLE IF NOT EXISTS config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
	}
	for _, ddl := range tables {
		if _, err := db.Exec(ddl); err != nil {
			return err
		}
	}

	// Add columns to existing tables - ignore errors if they already exist.
	alters := []string{
		"ALTER TABLE tasks ADD COLUMN parent_id INTEGER REFERENCES tasks(id)",
		"ALTER TABLE tasks ADD COLUMN position INTEGER",
		"ALTER TABLE tasks ADD COLUMN parallel INTEGER DEFAULT 0",
		"ALTER TABLE tasks ADD COLUMN status TEXT DEFAULT 'not_started'",
		"ALTER TABLE tasks ADD COLUMN description TEXT DEFAULT ''",
		"ALTER TABLE tasks ADD COLUMN verification TEXT DEFAULT ''",
		"ALTER TABLE tags ADD COLUMN ruletag INTEGER DEFAULT 0",
		"ALTER TABLE rules ADD COLUMN seq INTEGER DEFAULT 0",
		"ALTER TABLE tasks ADD COLUMN status_changed_at DATETIME DEFAULT CURRENT_TIMESTAMP",
		"ALTER TABLE tasks ADD COLUMN deadline DATETIME",
		"ALTER TABLE tasks ADD COLUMN recur TEXT",
		"ALTER TABLE tasks ADD COLUMN verify_status TEXT DEFAULT ''",
	}
	for _, stmt := range alters {
		db.Exec(stmt)
	}

	// Fix NULL status for rows that existed before the column was added.
	db.Exec("UPDATE tasks SET status = 'not_started' WHERE status IS NULL")

	// Backfill status_changed_at for rows that predate the column.
	db.Exec("UPDATE tasks SET status_changed_at = created_at WHERE status_changed_at IS NULL")

	// Backfill verify_status for existing tasks that have verification criteria.
	db.Exec("UPDATE tasks SET verify_status = 'pending' WHERE verification != '' AND (verify_status = '' OR verify_status IS NULL)")

	// Fix seq=0 for existing rules that predate the seq column.
	db.Exec(`UPDATE rules SET seq = (SELECT COUNT(*) FROM rules r2 WHERE r2.id <= rules.id) WHERE seq = 0 OR seq IS NULL`)

	// Migrate junction tables to add ON DELETE CASCADE.
	// SQLite requires recreating the table to change foreign key constraints.
	cascadeMigrations := []struct {
		table   string
		create  string
	}{
		{"task_tags", `CREATE TABLE task_tags_new (
			task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
			tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
			PRIMARY KEY (task_id, tag_id)
		)`},
		{"rule_tags", `CREATE TABLE rule_tags_new (
			rule_id INTEGER NOT NULL REFERENCES rules(id) ON DELETE CASCADE,
			tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
			PRIMARY KEY (rule_id, tag_id)
		)`},
		{"task_blockers", `CREATE TABLE task_blockers_new (
			task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
			blocker_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
			PRIMARY KEY (task_id, blocker_id)
		)`},
	}
	for _, m := range cascadeMigrations {
		// Check if migration is needed by looking for CASCADE in existing schema.
		var sql string
		err := db.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name=?", m.table).Scan(&sql)
		if err != nil || strings.Contains(sql, "CASCADE") {
			continue
		}
		db.Exec(m.create)
		db.Exec(fmt.Sprintf("INSERT INTO %s_new SELECT * FROM %s", m.table, m.table))
		db.Exec(fmt.Sprintf("DROP TABLE %s", m.table))
		db.Exec(fmt.Sprintf("ALTER TABLE %s_new RENAME TO %s", m.table, m.table))
	}

	return nil
}

// WriteTx runs fn inside an exclusive write transaction.
//
// The mutex lock ensures no read or write can run at the same time
// within this process (matters when butler runs as a long-lived
// MCP server with concurrent requests).
//
// The busy_timeout ensures cross-process safety - if another butler
// process is writing, this one waits up to 5 seconds for it to finish.
//
// If fn returns an error, the transaction is rolled back automatically.
func (s *Store) WriteTx(fn func(tx *sql.Tx) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// ReadTx runs fn inside a read transaction.
//
// RLock means multiple reads can proceed concurrently, but if a write
// is in progress, the read waits for it to finish first - so it always
// sees the latest committed data.
//
// The transaction is always rolled back at the end (reads don't modify data).
func (s *Store) ReadTx(fn func(tx *sql.Tx) error) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tx, err := s.db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	return fn(tx)
}

func (s *Store) Close() error {
	return s.db.Close()
}
