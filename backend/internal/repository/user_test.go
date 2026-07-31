package repository

import (
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
