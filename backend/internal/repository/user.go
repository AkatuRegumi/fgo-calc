package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fgo-calc-backend/internal/model"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

const userSchema = `
CREATE TABLE IF NOT EXISTS users (
	username TEXT PRIMARY KEY,
	password_hash BLOB NOT NULL,
	state TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS history (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT NOT NULL REFERENCES users(username) ON DELETE CASCADE,
	timestamp INTEGER NOT NULL,
	state TEXT NOT NULL,
	result TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_history_user_time
	ON history(username, timestamp DESC, id DESC);
CREATE TABLE IF NOT EXISTS app_metadata (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
`

type legacyUser struct {
	Username string               `json:"username"`
	Password string               `json:"password"`
	State    string               `json:"state"`
	History  []model.HistoryEntry `json:"history"`
}

func (r *Repository) initUserStore() error {
	dbPath := filepath.Join(r.dataDir, "users.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1)

	if _, err := db.Exec("PRAGMA journal_mode=WAL; PRAGMA synchronous=NORMAL; PRAGMA foreign_keys=ON; PRAGMA busy_timeout=5000;"); err != nil {
		db.Close()
		return err
	}
	if _, err := db.Exec(userSchema); err != nil {
		db.Close()
		return err
	}
	if err := os.Chmod(dbPath, 0600); err != nil {
		db.Close()
		return err
	}

	r.db = db
	if err := r.migrateLegacyUsers(); err != nil {
		db.Close()
		r.db = nil
		return err
	}
	return nil
}

func (r *Repository) migrateLegacyUsers() error {
	path := filepath.Join(r.dataDir, "users.json")
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	legacy := make(map[string]*legacyUser)
	decodeErr := json.NewDecoder(file).Decode(&legacy)
	closeErr := file.Close()
	if decodeErr != nil {
		return decodeErr
	}
	if closeErr != nil {
		return closeErr
	}

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var migrated int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM app_metadata WHERE key = 'legacy_users_migrated'`).Scan(&migrated); err != nil {
		return err
	}
	if migrated > 0 {
		if err := tx.Commit(); err != nil {
			return err
		}
		return os.Remove(path)
	}

	for key, user := range legacy {
		username := user.Username
		if username == "" {
			username = key
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO users(username, password_hash, state) VALUES (?, ?, ?)
			ON CONFLICT(username) DO NOTHING`, username, hash, user.State); err != nil {
			return err
		}
		for _, entry := range user.History {
			if _, err := tx.Exec(`INSERT INTO history(username, timestamp, state, result)
				SELECT ?, ?, ?, ? WHERE EXISTS (SELECT 1 FROM users WHERE username = ?)`,
				username, entry.Timestamp, entry.State, entry.Result, username); err != nil {
				return err
			}
		}
	}
	if _, err := tx.Exec(`INSERT INTO app_metadata(key, value) VALUES ('legacy_users_migrated', '1')`); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return os.Remove(path)
}

func (r *Repository) RegisterUser(username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	result, err := r.db.Exec(`INSERT INTO users(username, password_hash) VALUES (?, ?)
		ON CONFLICT(username) DO NOTHING`, username, hash)
	if err != nil {
		return err
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if inserted == 0 {
		return errors.New("username already exists")
	}
	return nil
}

func (r *Repository) LoginUser(username, password string) (*model.User, error) {
	var hash []byte
	user := &model.User{Username: username}
	err := r.db.QueryRow(`SELECT password_hash, state FROM users WHERE username = ?`, username).Scan(&hash, &user.State)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("user not found")
	}
	if err != nil {
		return nil, err
	}
	if bcrypt.CompareHashAndPassword(hash, []byte(password)) != nil {
		return nil, errors.New("invalid password")
	}
	return user, nil
}

func (r *Repository) SaveUserState(username, state string) error {
	result, err := r.db.Exec(`UPDATE users SET state = ? WHERE username = ?`, state, username)
	if err != nil {
		return err
	}
	return requireAffectedUser(result)
}

func (r *Repository) AddHistory(username, state, result string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	insert, err := tx.Exec(`INSERT INTO history(username, timestamp, state, result)
		SELECT ?, ?, ?, ? WHERE EXISTS (SELECT 1 FROM users WHERE username = ?)`,
		username, time.Now().UnixMilli(), state, result, username)
	if err != nil {
		return err
	}
	if err := requireAffectedUser(insert); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM history WHERE username = ? AND id NOT IN (
		SELECT id FROM history WHERE username = ? ORDER BY timestamp DESC, id DESC LIMIT 10
	)`, username, username); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) GetHistory(username string) ([]model.HistoryEntry, error) {
	var exists int
	if err := r.db.QueryRow(`SELECT 1 FROM users WHERE username = ?`, username).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("user not found")
	} else if err != nil {
		return nil, err
	}

	rows, err := r.db.Query(`SELECT timestamp, state, result FROM history
		WHERE username = ? ORDER BY timestamp DESC, id DESC LIMIT 10`, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	history := make([]model.HistoryEntry, 0, 10)
	for rows.Next() {
		var entry model.HistoryEntry
		if err := rows.Scan(&entry.Timestamp, &entry.State, &entry.Result); err != nil {
			return nil, err
		}
		history = append(history, entry)
	}
	return history, rows.Err()
}

func (r *Repository) GetUserState(username string) (string, error) {
	var state string
	err := r.db.QueryRow(`SELECT state FROM users WHERE username = ?`, username).Scan(&state)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errors.New("user not found")
	}
	return state, err
}

func requireAffectedUser(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errors.New("user not found")
	}
	return nil
}
