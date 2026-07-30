package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/delta-code/cli/pkg/models"
)

type ProjectMemory struct {
	db *sql.DB
}

type Session struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Title     string    `json:"title"`
}

type MemoryEntry struct {
	ID        int64           `json:"id"`
	SessionID int64           `json:"session_id"`
	Role      string          `json:"role"`
	Content   string          `json:"content"`
	Metadata  string          `json:"metadata"`
	CreatedAt time.Time       `json:"created_at"`
}

func NewProjectMemory() (*ProjectMemory, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".delta", "memory")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(dir, "project.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open memory db: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	if err := migrate(db); err != nil {
		return nil, err
	}

	return &ProjectMemory{db: db}, nil
}

func migrate(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL DEFAULT '',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS entries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id INTEGER NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		metadata TEXT DEFAULT '{}',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (session_id) REFERENCES sessions(id)
	);
	CREATE TABLE IF NOT EXISTS decisions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		description TEXT NOT NULL,
		rationale TEXT NOT NULL,
		alternatives TEXT DEFAULT '[]',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS mistakes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		description TEXT NOT NULL,
		root_cause TEXT NOT NULL,
		fix TEXT NOT NULL,
		file TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS coding_style (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		key TEXT UNIQUE NOT NULL,
		value TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_entries_session ON entries(session_id);
	`
	_, err := db.Exec(schema)
	return err
}

func (m *ProjectMemory) CreateSession(title string) (int64, error) {
	result, err := m.db.Exec("INSERT INTO sessions (title) VALUES (?)", title)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (m *ProjectMemory) AddEntry(sessionID int64, role string, content string, metadata map[string]any) error {
	metaJSON := "{}"
	if metadata != nil {
		b, _ := json.Marshal(metadata)
		metaJSON = string(b)
	}
	_, err := m.db.Exec(
		"INSERT INTO entries (session_id, role, content, metadata) VALUES (?, ?, ?, ?)",
		sessionID, role, content, metaJSON,
	)
	if err != nil {
		return err
	}
	_, err = m.db.Exec("UPDATE sessions SET updated_at = CURRENT_TIMESTAMP WHERE id = ?", sessionID)
	return err
}

func (m *ProjectMemory) GetHistory(sessionID int64, limit int) ([]MemoryEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := m.db.Query(
		"SELECT id, session_id, role, content, metadata, created_at FROM entries WHERE session_id = ? ORDER BY created_at DESC LIMIT ?",
		sessionID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []MemoryEntry
	for rows.Next() {
		var e MemoryEntry
		var createdAt string
		if err := rows.Scan(&e.ID, &e.SessionID, &e.Role, &e.Content, &e.Metadata, &createdAt); err != nil {
			return nil, err
		}
		e.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		entries = append(entries, e)
	}
	return entries, nil
}

func (m *ProjectMemory) AddDecision(title, description, rationale string, alternatives []string) error {
	altJSON, _ := json.Marshal(alternatives)
	_, err := m.db.Exec(
		"INSERT INTO decisions (title, description, rationale, alternatives) VALUES (?, ?, ?, ?)",
		title, description, rationale, string(altJSON),
	)
	return err
}

func (m *ProjectMemory) AddMistake(description, rootCause, fix, file string) error {
	_, err := m.db.Exec(
		"INSERT INTO mistakes (description, root_cause, fix, file) VALUES (?, ?, ?, ?)",
		description, rootCause, fix, file,
	)
	return err
}

func (m *ProjectMemory) SetCodingStyle(key, value string) error {
	_, err := m.db.Exec(
		"INSERT OR REPLACE INTO coding_style (key, value) VALUES (?, ?)",
		key, value,
	)
	return err
}

func (m *ProjectMemory) GetCodingStyle(key string) (string, error) {
	var value string
	err := m.db.QueryRow("SELECT value FROM coding_style WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (m *ProjectMemory) GetRecentSessions(limit int) ([]Session, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := m.db.Query("SELECT id, title, created_at, updated_at FROM sessions ORDER BY updated_at DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		var createdAt, updatedAt string
		if err := rows.Scan(&s.ID, &s.Title, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		s.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		s.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		sessions = append(sessions, s)
	}
	return sessions, nil
}

func (m *ProjectMemory) SearchEntries(query string, limit int) ([]MemoryEntry, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := m.db.Query(
		"SELECT id, session_id, role, content, metadata, created_at FROM entries WHERE content LIKE ? ORDER BY created_at DESC LIMIT ?",
		"%"+query+"%", limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []MemoryEntry
	for rows.Next() {
		var e MemoryEntry
		var createdAt string
		if err := rows.Scan(&e.ID, &e.SessionID, &e.Role, &e.Content, &e.Metadata, &createdAt); err != nil {
			return nil, err
		}
		e.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		entries = append(entries, e)
	}
	return entries, nil
}

func (m *ProjectMemory) Close() error {
	return m.db.Close()
}

func (m *ProjectMemory) ToChatHistory(sessionID int64) ([]models.Message, error) {
	entries, err := m.GetHistory(sessionID, 100)
	if err != nil {
		return nil, err
	}
	var msgs []models.Message
	for i := len(entries) - 1; i >= 0; i-- {
		msgs = append(msgs, models.Message{
			Role:    models.Role(entries[i].Role),
			Content: entries[i].Content,
		})
	}
	return msgs, nil
}
