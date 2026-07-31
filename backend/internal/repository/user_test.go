package repository

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"fgo-calc-backend/internal/model"
	"golang.org/x/crypto/bcrypt"
)

func TestUserStoreMigratesLegacyJSONAndHashesPassword(t *testing.T) {
	dataDir := t.TempDir()
	legacyPath := filepath.Join(dataDir, "users.json")
	legacy := map[string]*legacyUser{
		"alice": {
			Username: "alice",
			Password: "plain-password",
			State:    `{"cost":120}`,
			History: []model.HistoryEntry{{
				Timestamp: 1,
				State:     "state",
				Result:    "result",
			}},
		},
	}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath, data, 0600); err != nil {
		t.Fatal(err)
	}

	repo := &Repository{dataDir: dataDir}
	if err := repo.initUserStore(); err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("legacy JSON was not removed: %v", err)
	}
	var hash []byte
	if err := repo.db.QueryRow(`SELECT password_hash FROM users WHERE username = 'alice'`).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if string(hash) == "plain-password" {
		t.Fatal("password was stored as plaintext")
	}
	if err := bcrypt.CompareHashAndPassword(hash, []byte("plain-password")); err != nil {
		t.Fatalf("stored hash does not match password: %v", err)
	}

	user, err := repo.LoginUser("alice", "plain-password")
	if err != nil {
		t.Fatal(err)
	}
	if user.State != `{"cost":120}` {
		t.Fatalf("unexpected migrated state: %q", user.State)
	}
	history, err := repo.GetHistory("alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].Result != "result" {
		t.Fatalf("unexpected migrated history: %#v", history)
	}
}

func TestUserStoreKeepsLatestTenHistoryEntries(t *testing.T) {
	repo := &Repository{dataDir: t.TempDir()}
	if err := repo.initUserStore(); err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	if err := repo.RegisterUser("alice", "password"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 12; i++ {
		if err := repo.AddHistory("alice", "state", "result"); err != nil {
			t.Fatal(err)
		}
	}
	history, err := repo.GetHistory("alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 10 {
		t.Fatalf("expected 10 history entries, got %d", len(history))
	}
}

func TestPinnedHistoryDoesNotUseRollingHistoryLimit(t *testing.T) {
	repo := &Repository{dataDir: t.TempDir()}
	if err := repo.initUserStore(); err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	if err := repo.RegisterUser("alice", "password"); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 15; i++ {
		if err := repo.AddHistory("alice", "pinned-state", "result"); err != nil {
			t.Fatal(err)
		}
		var id int64
		if err := repo.db.QueryRow(`SELECT id FROM history WHERE username = 'alice' ORDER BY id DESC LIMIT 1`).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if err := repo.SetHistoryPinned("alice", id, true); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 12; i++ {
		if err := repo.AddHistory("alice", "rolling-state", "result"); err != nil {
			t.Fatal(err)
		}
	}

	history, err := repo.GetHistory("alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 25 {
		t.Fatalf("expected 15 pinned and 10 rolling entries, got %d", len(history))
	}
	pinned := 0
	for _, entry := range history {
		if entry.Pinned {
			pinned++
		}
	}
	if pinned != 15 {
		t.Fatalf("expected 15 pinned entries, got %d", pinned)
	}

	var rollingID int64
	if err := repo.db.QueryRow(`SELECT id FROM history WHERE username = 'alice' AND pinned = 0 LIMIT 1`).Scan(&rollingID); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetHistoryPinned("alice", rollingID, true); err == nil || err.Error() != "pinned history limit reached" {
		t.Fatalf("expected pinned history limit error, got %v", err)
	}
	if err := repo.SetHistoryPinned("alice", history[14].ID, false); err != nil {
		t.Fatal(err)
	}
	history, err = repo.GetHistory("alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 24 {
		t.Fatalf("expected unpinned old entry to be pruned, got %d entries", len(history))
	}
}

func TestRenameHistory(t *testing.T) {
	repo := &Repository{dataDir: t.TempDir()}
	if err := repo.initUserStore(); err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	if err := repo.RegisterUser("alice", "password"); err != nil {
		t.Fatal(err)
	}
	if err := repo.AddHistory("alice", "state", "result"); err != nil {
		t.Fatal(err)
	}
	var id int64
	if err := repo.db.QueryRow(`SELECT id FROM history WHERE username = 'alice'`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if err := repo.RenameHistory("alice", id, "周回方案"); err != nil {
		t.Fatal(err)
	}
	history, err := repo.GetHistory("alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].Name != "周回方案" {
		t.Fatalf("unexpected renamed history: %#v", history)
	}
}

func TestUserStoreUpgradesExistingHistorySchema(t *testing.T) {
	dataDir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dataDir, "users.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		CREATE TABLE users (username TEXT PRIMARY KEY, password_hash BLOB NOT NULL, state TEXT NOT NULL DEFAULT '');
		CREATE TABLE history (id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT NOT NULL, timestamp INTEGER NOT NULL, state TEXT NOT NULL, result TEXT NOT NULL);
		INSERT INTO users(username, password_hash) VALUES ('alice', 'hash');
		INSERT INTO history(username, timestamp, state, result) VALUES ('alice', 1, 'state', 'result');
	`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	repo := &Repository{dataDir: dataDir}
	if err := repo.initUserStore(); err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	history, err := repo.GetHistory("alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].Name != "" || history[0].Pinned {
		t.Fatalf("unexpected upgraded history: %#v", history)
	}
}
