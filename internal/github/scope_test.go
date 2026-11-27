package github

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveScopeFrom(t *testing.T) {
	t.Run("config user", func(t *testing.T) {
		s := ResolveScopeFrom("api.github.com", "gho_secret", "oauth_token", "rokuosan")
		assert.Equal(t, Scope{Host: "github.com", User: "rokuosan"}, s)
		assert.Equal(t, filepath.Join("github.com", "rokuosan"), s.CacheSubdir())
	})

	t.Run("enterprise host and fallback user", func(t *testing.T) {
		s := ResolveScopeFrom("GHE.Example.COM", "tok", "oauth_token", "")
		assert.Equal(t, Scope{Host: "ghe.example.com", User: "default"}, s)
		assert.Equal(t, filepath.Join("ghe.example.com", "default"), s.CacheSubdir())
	})

	t.Run("env token hashes token instead of using raw token or cfgUser", func(t *testing.T) {
		s1 := ResolveScopeFrom("github.com", "ghp_super_secret_1", "GH_TOKEN", "rokuosan")
		s2 := ResolveScopeFrom("github.com", "ghp_super_secret_2", "GITHUB_TOKEN", "rokuosan")

		assert.True(t, strings.HasPrefix(s1.User, "env-"))
		assert.Len(t, s1.User, len("env-")+12)
		assert.NotContains(t, s1.User, "ghp_super_secret_1")
		assert.NotEqual(t, s1.User, s2.User)
	})
}

func TestScope_CacheSubdir_SanitizesUnsafeSegments(t *testing.T) {
	s1 := Scope{Host: "../../etc", User: "../alice/bob"}
	sub1 := s1.CacheSubdir()
	assert.NotContains(t, sub1, "..")
	parts1 := strings.Split(sub1, string(filepath.Separator))
	assert.Len(t, parts1, 2)
	assert.False(t, strings.HasPrefix(parts1[0], "."))
	assert.False(t, strings.HasPrefix(parts1[1], "."))

	s2 := Scope{Host: "../etc", User: "alice:bob"}
	s3 := Scope{Host: "../etc", User: "alice/bob"}
	assert.NotEqual(t, s2.CacheSubdir(), s3.CacheSubdir())

	sEmpty := Scope{Host: "..", User: "."}
	assert.Equal(t, filepath.Join("github.com", "default"), sEmpty.CacheSubdir())
}
