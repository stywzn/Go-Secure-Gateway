package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// RouteConfig describes a single upstream route mounted on the gateway.
type RouteConfig struct {
	PathPrefix string `yaml:"path_prefix"`

	// TargetURL is the legacy single-backend field. Kept for backward
	// compatibility; prefer Targets for load-balanced routes.
	TargetURL string `yaml:"target_url"`

	// Targets is the list of upstreams to round-robin across.
	Targets []string `yaml:"targets"`

	// StripPrefix removes PathPrefix from the request path before it is
	// forwarded upstream (e.g. /storage/foo -> /foo).
	StripPrefix bool `yaml:"strip_prefix"`
}

// Backends returns the effective list of upstream URLs for the route,
// merging the legacy TargetURL with the Targets list.
func (r RouteConfig) Backends() []string {
	var out []string
	if r.TargetURL != "" {
		out = append(out, r.TargetURL)
	}
	out = append(out, r.Targets...)
	return out
}

type ServerConfig struct {
	Port string `yaml:"port"`
	// Timeouts are expressed in seconds; zero means "use the default".
	ReadTimeoutSeconds  int `yaml:"read_timeout_seconds"`
	WriteTimeoutSeconds int `yaml:"write_timeout_seconds"`
	IdleTimeoutSeconds  int `yaml:"idle_timeout_seconds"`
	// UpstreamTimeoutSeconds bounds how long the gateway waits for a backend
	// response before returning 504. Zero means "use the default".
	UpstreamTimeoutSeconds int `yaml:"upstream_timeout_seconds"`
}

type JWTConfig struct {
	Secret string `yaml:"secret"`
}

type RateLimitConfig struct {
	RPS   float64 `yaml:"rps"`
	Burst int     `yaml:"burst"`
}

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	JWT       JWTConfig       `yaml:"jwt"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`

	// Debug, when true, exposes helper endpoints such as /debug/token.
	// It must never be enabled in production.
	Debug bool `yaml:"debug"`

	Routes []RouteConfig `yaml:"routes"`
}

// LoadConfig reads, parses, normalizes and validates the gateway config.
// It returns an error instead of exiting so the caller controls the failure
// path and so the loader can be unit-tested.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %q: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	cfg.applyDefaults()

	// Allow the JWT secret to be injected from the environment (e.g. a
	// Kubernetes Secret) rather than living in plaintext in the config file.
	if envSecret := os.Getenv("JWT_SECRET"); envSecret != "" {
		cfg.JWT.Secret = envSecret
	}

	if err := cfg.normalizeAndValidate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Server.Port == "" {
		c.Server.Port = ":8080"
	}
	if c.Server.ReadTimeoutSeconds == 0 {
		c.Server.ReadTimeoutSeconds = 15
	}
	if c.Server.WriteTimeoutSeconds == 0 {
		c.Server.WriteTimeoutSeconds = 15
	}
	if c.Server.IdleTimeoutSeconds == 0 {
		c.Server.IdleTimeoutSeconds = 60
	}
	if c.Server.UpstreamTimeoutSeconds == 0 {
		c.Server.UpstreamTimeoutSeconds = 30
	}
	if c.RateLimit.RPS == 0 {
		c.RateLimit.RPS = 2
	}
	if c.RateLimit.Burst == 0 {
		c.RateLimit.Burst = 5
	}
}

func (c *Config) normalizeAndValidate() error {
	if c.JWT.Secret == "" {
		return fmt.Errorf("jwt.secret must be set (via config or JWT_SECRET env)")
	}

	if len(c.Routes) == 0 {
		return fmt.Errorf("at least one route must be configured")
	}

	seen := make(map[string]struct{}, len(c.Routes))
	for i := range c.Routes {
		r := &c.Routes[i]

		// Gin requires paths to begin with '/'. Normalize instead of
		// crashing on a config typo like "storage".
		prefix := strings.TrimSpace(r.PathPrefix)
		if prefix == "" {
			return fmt.Errorf("route %d: path_prefix must not be empty", i)
		}
		if !strings.HasPrefix(prefix, "/") {
			prefix = "/" + prefix
		}
		// Trim a trailing slash so "/storage/" and "/storage" collapse.
		if len(prefix) > 1 {
			prefix = strings.TrimRight(prefix, "/")
		}
		r.PathPrefix = prefix

		if _, dup := seen[prefix]; dup {
			return fmt.Errorf("route %d: duplicate path_prefix %q", i, prefix)
		}
		seen[prefix] = struct{}{}

		if len(r.Backends()) == 0 {
			return fmt.Errorf("route %q: at least one target (target_url or targets) is required", prefix)
		}
	}
	return nil
}
