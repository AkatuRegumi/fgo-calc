package repository

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAnnouncementsSortsNewestFirst(t *testing.T) {
	path := filepath.Join(t.TempDir(), "announcement.txt")
	content := "旧公告\n2026-07-01\n旧内容\n------\n新公告\n2026-08-01\n第一行\n第二行\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	announcements, err := loadAnnouncements(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(announcements) != 2 || announcements[0].Title != "新公告" {
		t.Fatalf("unexpected announcements: %#v", announcements)
	}
	if announcements[0].Content != "第一行\n第二行" {
		t.Fatalf("unexpected multiline content: %q", announcements[0].Content)
	}
}

func TestLoadAnnouncementsRejectsInvalidDate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "announcement.txt")
	if err := os.WriteFile(path, []byte("标题\n2026/08/01\n内容\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadAnnouncements(path); err == nil {
		t.Fatal("expected invalid date error")
	}
}

func TestLoadAnnouncementsAllowsMissingFile(t *testing.T) {
	announcements, err := loadAnnouncements(filepath.Join(t.TempDir(), "announcement.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(announcements) != 0 {
		t.Fatalf("expected no announcements, got %#v", announcements)
	}
}
