package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jeroen/make-ics-go/pkg/model"
)

func TestParseEnvLine(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantKey string
		wantVal string
		wantOK  bool
	}{
		{name: "empty", line: "   ", wantOK: false},
		{name: "comment", line: "# hello", wantOK: false},
		{name: "plain", line: "HOST=https://caldav.icloud.com", wantKey: "HOST", wantVal: "https://caldav.icloud.com", wantOK: true},
		{name: "export quoted", line: "export ID=\"jeroen@example.com\"", wantKey: "ID", wantVal: "jeroen@example.com", wantOK: true},
		{name: "single quoted", line: "PASSWORD='secret'", wantKey: "PASSWORD", wantVal: "secret", wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, val, ok := parseEnvLine(tt.line)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if key != tt.wantKey {
				t.Fatalf("key = %q, want %q", key, tt.wantKey)
			}
			if val != tt.wantVal {
				t.Fatalf("val = %q, want %q", val, tt.wantVal)
			}
		})
	}
}

func TestLoadDotEnv_DoesNotOverrideExistingEnv(t *testing.T) {
	t.Setenv("ID", "existing-user")

	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "ID=file-user\nPASSWORD=file-pass\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	stats, err := loadDotEnv(path)
	if err != nil {
		t.Fatalf("loadDotEnv: %v", err)
	}
	if !stats.Found {
		t.Fatal("stats.Found = false, want true")
	}
	if stats.Loaded != 1 {
		t.Fatalf("stats.Loaded = %d, want 1", stats.Loaded)
	}
	if stats.Skipped != 1 {
		t.Fatalf("stats.Skipped = %d, want 1", stats.Skipped)
	}

	if got := os.Getenv("ID"); got != "existing-user" {
		t.Fatalf("ID = %q, want %q", got, "existing-user")
	}
	if got := os.Getenv("PASSWORD"); got != "file-pass" {
		t.Fatalf("PASSWORD = %q, want %q", got, "file-pass")
	}
}

func TestLoadDotEnv_MissingFile(t *testing.T) {
	stats, err := loadDotEnv(filepath.Join(t.TempDir(), ".env"))
	if err != nil {
		t.Fatalf("loadDotEnv: %v", err)
	}
	if stats.Found {
		t.Fatal("stats.Found = true, want false")
	}
	if stats.Loaded != 0 || stats.Skipped != 0 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestResolveCalDAVSettings_FallbackFromDotEnvKeys(t *testing.T) {
	t.Setenv("HOST", "https://caldav.icloud.com")
	t.Setenv("ID", "jeroen@houtzager.org")
	t.Setenv("PASSWORD", "app-pass")
	t.Setenv("CALDAV_USERNAME", "")
	t.Setenv("CALDAV_PASSWORD", "")

	cfg := model.CalDAVConfig{
		URL:                 "https://example.invalid",
		UsernameEnv:         "CALDAV_USERNAME",
		PasswordEnv:         "CALDAV_PASSWORD",
		CalendarDisplayName: "Cal",
	}

	resolved, user, pass, err := resolveCalDAVSettings(cfg)
	if err != nil {
		t.Fatalf("resolveCalDAVSettings: %v", err)
	}
	if resolved.URL != "https://caldav.icloud.com" {
		t.Fatalf("resolved.URL = %q, want %q", resolved.URL, "https://caldav.icloud.com")
	}
	if user != "jeroen@houtzager.org" {
		t.Fatalf("user = %q, want %q", user, "jeroen@houtzager.org")
	}
	if pass != "app-pass" {
		t.Fatalf("pass = %q, want %q", pass, "app-pass")
	}
}

func TestResolveCalDAVSettings_PrefersConfiguredEnvVars(t *testing.T) {
	t.Setenv("HOST", "https://caldav.icloud.com")
	t.Setenv("ID", "fallback-id")
	t.Setenv("PASSWORD", "fallback-pass")
	t.Setenv("CALDAV_USERNAME", "configured-user")
	t.Setenv("CALDAV_PASSWORD", "configured-pass")

	cfg := model.CalDAVConfig{
		URL:                 "https://example.invalid",
		UsernameEnv:         "CALDAV_USERNAME",
		PasswordEnv:         "CALDAV_PASSWORD",
		CalendarDisplayName: "Cal",
	}

	_, user, pass, err := resolveCalDAVSettings(cfg)
	if err != nil {
		t.Fatalf("resolveCalDAVSettings: %v", err)
	}
	if user != "configured-user" {
		t.Fatalf("user = %q, want %q", user, "configured-user")
	}
	if pass != "configured-pass" {
		t.Fatalf("pass = %q, want %q", pass, "configured-pass")
	}
}

func TestResolveCalDAVSettings_InvalidHost(t *testing.T) {
	t.Setenv("HOST", "://bad")
	t.Setenv("ID", "u")
	t.Setenv("PASSWORD", "p")

	cfg := model.CalDAVConfig{
		URL:                 "https://example.invalid",
		UsernameEnv:         "CALDAV_USERNAME",
		PasswordEnv:         "CALDAV_PASSWORD",
		CalendarDisplayName: "Cal",
	}

	_, _, _, err := resolveCalDAVSettings(cfg)
	if err == nil {
		t.Fatal("expected invalid host error, got nil")
	}
}

func TestParseDateBound(t *testing.T) {
	start, err := parseDateBound("2026-06-15", false)
	if err != nil {
		t.Fatalf("parseDateBound start: %v", err)
	}
	if !start.Equal(time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("start = %v", start)
	}

	end, err := parseDateBound("2026-06-15", true)
	if err != nil {
		t.Fatalf("parseDateBound end: %v", err)
	}
	if !end.Equal(time.Date(2026, 6, 15, 23, 59, 59, 999999999, time.UTC)) {
		t.Fatalf("end = %v", end)
	}
}
