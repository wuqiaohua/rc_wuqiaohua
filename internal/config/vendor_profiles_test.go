package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadVendorProfilesFromTOML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vendor_profiles.toml")
	content := []byte(`
[vendors.crm]
target_url = "https://example.com/crm"
method = "POST"
max_retries = 7

[vendors.crm.default_headers]
Content-Type = "application/json"
X-Source = "notify"
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	profiles, err := LoadVendorProfiles(path)
	if err != nil {
		t.Fatalf("load profiles: %v", err)
	}
	profile := profiles["crm"]
	if profile.Vendor != "crm" {
		t.Fatalf("expected vendor crm, got %s", profile.Vendor)
	}
	if profile.TargetURL != "https://example.com/crm" {
		t.Fatalf("expected target url from toml, got %s", profile.TargetURL)
	}
	if profile.Method != "POST" {
		t.Fatalf("expected method POST, got %s", profile.Method)
	}
	if profile.MaxRetries != 7 {
		t.Fatalf("expected max retries 7, got %d", profile.MaxRetries)
	}
	if profile.DefaultHeaders["X-Source"] != "notify" {
		t.Fatalf("expected default header from toml, got %#v", profile.DefaultHeaders)
	}
}

func TestLoadVendorProfilesRejectsEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vendor_profiles.toml")
	if err := os.WriteFile(path, []byte(``), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := LoadVendorProfiles(path); err == nil {
		t.Fatalf("expected empty config to be rejected")
	}
}
