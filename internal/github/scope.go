package github

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"

	"github.com/cli/go-gh/v2/pkg/auth"
	"github.com/cli/go-gh/v2/pkg/config"
)

// Scope identifies the GitHub host and authenticated account used to isolate local caches.
type Scope struct {
	Host string
	User string
}

// ResolveScope detects the active gh host and account without making network calls.
func ResolveScope() Scope {
	host, _ := auth.DefaultHost()
	normalizedHost := auth.NormalizeHostname(strings.TrimSpace(host))
	if normalizedHost == "" {
		normalizedHost = "github.com"
	}

	token, source := auth.TokenFromEnvOrConfig(normalizedHost)

	var cfgUser string
	if cfg, err := config.Read(nil); err == nil && cfg != nil {
		if u, err := cfg.Get([]string{"hosts", normalizedHost, "user"}); err == nil {
			cfgUser = u
		}
	}

	return ResolveScopeFrom(normalizedHost, token, source, cfgUser)
}

// ResolveScopeFrom builds a Scope from explicit auth/config inputs.
func ResolveScopeFrom(host, token, tokenSource, cfgUser string) Scope {
	h := auth.NormalizeHostname(strings.TrimSpace(host))
	if h == "" {
		h = "github.com"
	}

	var u string
	switch tokenSource {
	case "GH_TOKEN", "GITHUB_TOKEN", "GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN":
		if token != "" {
			sum := sha256.Sum256([]byte(token))
			u = "env-" + hex.EncodeToString(sum[:])[:12]
		}
	}
	if u == "" {
		if trimmed := strings.TrimSpace(cfgUser); trimmed != "" {
			u = trimmed
		} else {
			u = "default"
		}
	}

	return Scope{
		Host: h,
		User: u,
	}
}

// CacheSubdir returns a sanitized relative path "<host>/<user>" safe for filesystem directories.
func (s Scope) CacheSubdir() string {
	return filepath.Join(
		sanitizeSegment(s.Host, "github.com"),
		sanitizeSegment(s.User, "default"),
	)
}

func sanitizeSegment(raw, fallback string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "." || trimmed == ".." {
		return fallback
	}

	var b strings.Builder
	b.Grow(len(trimmed))
	modified := trimmed != raw
	for i, r := range trimmed {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_':
			b.WriteRune(r)
		case r == '.':
			if i == 0 {
				b.WriteByte('_')
				modified = true
			} else {
				b.WriteByte('.')
			}
		default:
			b.WriteByte('_')
			modified = true
		}
	}

	cleaned := b.String()
	if strings.Contains(cleaned, "..") {
		cleaned = strings.ReplaceAll(cleaned, "..", "__")
		modified = true
	}

	if cleaned == "" || cleaned == "." || cleaned == ".." {
		cleaned = fallback
		modified = true
	}

	if modified {
		sum := sha256.Sum256([]byte(raw))
		return cleaned + "-" + hex.EncodeToString(sum[:])[:8]
	}

	return cleaned
}
