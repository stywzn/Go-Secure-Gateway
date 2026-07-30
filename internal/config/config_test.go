package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}

func TestLoadConfig_NormalizesPrefixAndDefaults(t *testing.T) {
	path := writeTempConfig(t, `
jwt:
  secret: "s"
routes:
  - path_prefix: "storage"
    target_url: "http://localhost:8082"
  - path_prefix: "/compute/"
    target_url: "http://localhost:8083"
`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Routes[0].PathPrefix != "/storage" {
		t.Errorf("prefix not normalized: got %q, want /storage", cfg.Routes[0].PathPrefix)
	}
	if cfg.Routes[1].PathPrefix != "/compute" {
		t.Errorf("trailing slash not trimmed: got %q, want /compute", cfg.Routes[1].PathPrefix)
	}
	if cfg.Server.Port != ":8080" {
		t.Errorf("default port not applied: got %q", cfg.Server.Port)
	}
	if cfg.RateLimit.RPS != 2 || cfg.RateLimit.Burst != 5 {
		t.Errorf("rate limit defaults not applied: %+v", cfg.RateLimit)
	}
}

func TestLoadConfig_EnvSecretOverride(t *testing.T) {
	path := writeTempConfig(t, `
jwt:
  secret: "file-secret"
routes:
  - path_prefix: "/a"
    target_url: "http://localhost:9000"
`)
	t.Setenv("JWT_SECRET", "env-secret")

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.JWT.Secret != "env-secret" {
		t.Errorf("env secret did not override: got %q", cfg.JWT.Secret)
	}
}

func TestLoadConfig_Validation(t *testing.T) {
	cases := map[string]string{
		"missing secret": `
routes:
  - path_prefix: "/a"
    target_url: "http://localhost:9000"
`,
		"no routes": `
jwt:
  secret: "s"
`,
		"route without target": `
jwt:
  secret: "s"
routes:
  - path_prefix: "/a"
`,
		"duplicate prefix": `
jwt:
  secret: "s"
routes:
  - path_prefix: "/a"
    target_url: "http://x:1"
  - path_prefix: "a"
    target_url: "http://y:2"
`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			// Ensure a leaked env var doesn't satisfy the missing-secret case.
			t.Setenv("JWT_SECRET", "")
			path := writeTempConfig(t, body)
			if _, err := LoadConfig(path); err == nil {
				t.Errorf("expected validation error for %q", name)
			}
		})
	}
}

func TestRouteConfig_Backends(t *testing.T) {
	r := RouteConfig{TargetURL: "http://a", Targets: []string{"http://b", "http://c"}}
	got := r.Backends()
	if len(got) != 3 || got[0] != "http://a" || got[2] != "http://c" {
		t.Errorf("Backends() = %v, want merged list", got)
	}
}
