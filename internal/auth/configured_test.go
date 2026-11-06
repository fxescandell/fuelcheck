package auth_test

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/fxescandell/fuelcheck/internal/auth"
)

func TestConfiguredCredentialsOverrideDiscovery(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ZAI_API_KEY", "detected-zai")
	for path, data := range map[string]string{
		".codex/auth.json": `{"tokens":{"access_token":"detected-codex","account_id":"account"}}`,
		".mmx/config.json": `{"api_key":"detected-minimax"}`,
	} {
		p := filepath.Join(home, path)
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tt := range []struct {
		name     string
		get      func(string) (string, error)
		detected string
	}{
		{"codex", func(key string) (string, error) {
			c, e := auth.GetCodexCredentials(key)
			if e != nil {
				return "", e
			}
			return c.AccessToken, nil
		}, "detected-codex"},
		{"minimax", func(key string) (string, error) {
			c, e := auth.GetMiniMaxCredentials(key)
			if e != nil {
				return "", e
			}
			return c.APIKey, nil
		}, "detected-minimax"},
		{"z.ai", func(key string) (string, error) {
			c, e := auth.GetZaiCredentials(key)
			if e != nil {
				return "", e
			}
			return c.APIKey, nil
		}, "detected-zai"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, tc := range []struct{ key, want string }{{"configured", "configured"}, {"", tt.detected}} {
				got, err := tt.get(tc.key)
				if err != nil || got != tc.want {
					t.Fatalf("precedencia incorrecta: got=%q err=%v", got, err)
				}
			}
		})
	}
}

func TestConfiguredCodexTokenSuppliesItsOwnAccount(t *testing.T) {
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"https://api.openai.com/auth":{"chatgpt_account_id":"configured-account"}}`))
	token := "header." + payload + ".signature"
	creds, err := auth.GetCodexCredentials(token)
	if err != nil {
		t.Fatal(err)
	}
	if creds.AccessToken != token || creds.AccountID != "configured-account" {
		t.Fatal("el token configurado no conservó su cuenta")
	}
}
