package context

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Store persists project contexts
type Store struct {
	db *sql.DB
}

// NewStore creates/opens SQLite store
func NewStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	if err := initSchema(db); err != nil {
		db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

// Close closes database
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Save persists context
func (s *Store) Save(p *ProjectContext) error {
	data, err := p.ToJSON()
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		INSERT INTO contexts (path, name, type, data, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			name = excluded.name,
			type = excluded.type,
			data = excluded.data,
			updated_at = excluded.updated_at
	`, p.Path, p.Name, p.Type, string(data), time.Now())
	return err
}

// Load retrieves context by path
func (s *Store) Load(path string) (*ProjectContext, error) {
	var data string
	err := s.db.QueryRow(`SELECT data FROM contexts WHERE path = ?`, path).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return FromJSON([]byte(data))
}

// List returns all stored contexts
func (s *Store) List() ([]*ProjectContext, error) {
	rows, err := s.db.Query(`SELECT data FROM contexts ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contexts []*ProjectContext
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			continue
		}
		if p, err := FromJSON([]byte(data)); err == nil {
			contexts = append(contexts, p)
		}
	}
	return contexts, nil
}

// Delete removes context
func (s *Store) Delete(path string) error {
	_, err := s.db.Exec(`DELETE FROM contexts WHERE path = ?`, path)
	return err
}

func initSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS contexts (
			path TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			data TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_type ON contexts(type);
	`)
	return err
}

// DefaultStorePath returns default store location
func DefaultStorePath() string {
	home := os.Getenv("HOME")
	if home == "" {
		home = "."
	}
	return filepath.Join(home, ".config", "otter", "contexts.db")
}
