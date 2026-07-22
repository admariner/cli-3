package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestFilterProjectConfigAllowlist(t *testing.T) {
	raw := map[string]interface{}{
		"org":                "acme",
		"database":           "shop",
		"branch":             "main",
		"api-url":            "https://evil.example.com",
		"api-token":          "stolen",
		"service-token":      "tok",
		"service-token-id":   "id",
		"service-token-name": "legacy",
		"format":             "json",
		"debug":              true,
	}
	allowed, ignored := FilterProjectConfig(raw)
	if len(allowed) != 3 {
		t.Fatalf("allowed = %#v, want org/database/branch only", allowed)
	}
	if allowed["org"] != "acme" || allowed["database"] != "shop" || allowed["branch"] != "main" {
		t.Fatalf("allowed values = %#v", allowed)
	}
	for _, key := range []string{"api-url", "api-token", "service-token", "service-token-id", "service-token-name", "format", "debug"} {
		found := false
		for _, ig := range ignored {
			if ig == key {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected %q in ignored keys %#v", key, ignored)
		}
	}
}

func TestFilterProjectConfigCaseInsensitive(t *testing.T) {
	allowed, ignored := FilterProjectConfig(map[string]interface{}{
		"ORG":     "acme",
		"Api-Url": "https://evil.example.com",
	})
	if allowed["org"] != "acme" {
		t.Fatalf("ORG should map to org allowlist entry, got %#v", allowed)
	}
	if len(ignored) != 1 || ignored[0] != "Api-Url" {
		t.Fatalf("ignored = %#v", ignored)
	}
}

func TestMergeProjectConfigIgnoresSensitiveKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, projectConfigName)
	content := "" +
		"org: victim-org\n" +
		"database: shop\n" +
		"branch: main\n" +
		"api-url: https://evil.example.com\n" +
		"api-token: attacker-token\n" +
		"service-token: pscale_tkn_evil\n" +
		"service-token-id: abc123xyz999\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	v := viper.New()
	v.Set("api-url", "https://api.planetscale.com/")
	ignored, err := MergeProjectConfig(v, dir)
	if err != nil {
		t.Fatal(err)
	}
	if v.GetString("org") != "victim-org" {
		t.Fatalf("org = %q", v.GetString("org"))
	}
	if v.GetString("database") != "shop" || v.GetString("branch") != "main" {
		t.Fatalf("database/branch not merged: %q %q", v.GetString("database"), v.GetString("branch"))
	}
	if got := v.GetString("api-url"); got != "https://api.planetscale.com/" {
		t.Fatalf("api-url overridden by project config: %q", got)
	}
	if v.IsSet("api-token") && v.GetString("api-token") != "" {
		t.Fatalf("api-token must not come from project config, got %q", v.GetString("api-token"))
	}
	if v.GetString("service-token") != "" || v.GetString("service-token-id") != "" {
		t.Fatalf("service token fields must not come from project config")
	}
	joined := strings.Join(ignored, ",")
	for _, key := range []string{"api-url", "api-token", "service-token", "service-token-id"} {
		if !strings.Contains(joined, key) {
			t.Fatalf("ignored=%v, want %q reported", ignored, key)
		}
	}
}

func TestMergeProjectConfigMissingFile(t *testing.T) {
	v := viper.New()
	ignored, err := MergeProjectConfig(v, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(ignored) != 0 {
		t.Fatalf("ignored = %v", ignored)
	}
}

func TestMergeProjectConfigDoesNotOverrideHomeApiURL(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, projectConfigName), []byte("org: acme\napi-url: https://evil.example.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	v := viper.New()
	// Simulate a user home config / prior value that must win over project files.
	v.Set("api-url", "https://api.planetscale.com/")
	if _, err := MergeProjectConfig(v, dir); err != nil {
		t.Fatal(err)
	}
	if got := v.GetString("api-url"); got != "https://api.planetscale.com/" {
		t.Fatalf("project api-url leaked into viper: %q", got)
	}
}
