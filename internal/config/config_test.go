package config

import (
	"os"
	"testing"
)

func TestLoadDBPath(t *testing.T) {
	t.Run("defaults to tasks.db when DB_PATH unset", func(t *testing.T) {
		os.Unsetenv("DB_PATH")
		cfg := Load()
		if cfg.DBPath != "tasks.db" {
			t.Errorf("DBPath = %q, want tasks.db", cfg.DBPath)
		}
	})

	t.Run("reads DB_PATH from env", func(t *testing.T) {
		os.Setenv("DB_PATH", "/data/tasks.db")
		defer os.Unsetenv("DB_PATH")
		cfg := Load()
		if cfg.DBPath != "/data/tasks.db" {
			t.Errorf("DBPath = %q, want /data/tasks.db", cfg.DBPath)
		}
	})
}

func TestLoadZitadelFields(t *testing.T) {
	t.Run("reads all Zitadel env vars", func(t *testing.T) {
		os.Setenv("ZITADEL_ISSUER", "https://example.zitadel.cloud")
		os.Setenv("ZITADEL_AUTH_URL", "https://example.zitadel.cloud/oauth/v2/authorize")
		os.Setenv("ZITADEL_TOKEN_URL", "https://example.zitadel.cloud/oauth/v2/token")
		os.Setenv("ZITADEL_JWKS_URL", "https://example.zitadel.cloud/oauth/v2/keys")
		defer func() {
			os.Unsetenv("ZITADEL_ISSUER")
			os.Unsetenv("ZITADEL_AUTH_URL")
			os.Unsetenv("ZITADEL_TOKEN_URL")
			os.Unsetenv("ZITADEL_JWKS_URL")
		}()
		cfg := Load()
		if cfg.ZitadelIssuer != "https://example.zitadel.cloud" {
			t.Errorf("ZitadelIssuer = %q", cfg.ZitadelIssuer)
		}
		if cfg.ZitadelAuthURL != "https://example.zitadel.cloud/oauth/v2/authorize" {
			t.Errorf("ZitadelAuthURL = %q", cfg.ZitadelAuthURL)
		}
		if cfg.ZitadelTokenURL != "https://example.zitadel.cloud/oauth/v2/token" {
			t.Errorf("ZitadelTokenURL = %q", cfg.ZitadelTokenURL)
		}
		if cfg.ZitadelJWKSURL != "https://example.zitadel.cloud/oauth/v2/keys" {
			t.Errorf("ZitadelJWKSURL = %q", cfg.ZitadelJWKSURL)
		}
	})
}
