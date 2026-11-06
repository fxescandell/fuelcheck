package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func write(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "config.json")); err == nil {
		t.Fatal("se esperaba un error al faltar el fichero")
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	if _, err := Load(write(t, `{"codex":`)); err == nil {
		t.Fatal("se esperaba un error por JSON inválido")
	}
}

func TestLoadNotAnObject(t *testing.T) {
	for _, content := range []string{`[1,2]`, `null`, `"codex"`} {
		if _, err := Load(write(t, content)); err == nil {
			t.Fatalf("se esperaba un error por no ser un objeto: %s", content)
		}
	}
}

func TestLoadUnknownProvider(t *testing.T) {
	if _, err := Load(write(t, `{"claude":{"enabled":true}}`)); err == nil {
		t.Fatal("se esperaba un error por proveedor desconocido")
	}
}

func TestLoadProviders(t *testing.T) {
	content := `{"codex":{"enabled":true,"api_key":"codex-key","endpoint":"http://localhost:1"},"minimax":{"enabled":true,"api_key":""},"z.ai":{"enabled":false,"api_key":"zai-key"}}`
	cfg, err := Load(write(t, content))
	if err != nil {
		t.Fatal(err)
	}
	codex := cfg.Provider("codex")
	if !codex.Enabled || codex.APIKey != "codex-key" || codex.Endpoint != "http://localhost:1" {
		t.Fatal("codex cargado incorrectamente")
	}
	if !cfg.Provider("minimax").Enabled || cfg.Provider("minimax").APIKey != "" {
		t.Fatal("minimax cargado incorrectamente")
	}
	if cfg.Provider("z.ai").Enabled || cfg.Provider("z.ai").APIKey != "zai-key" {
		t.Fatal("z.ai cargado incorrectamente")
	}
}

func TestIntervalDefaultsToOneMinute(t *testing.T) {
	cfg, err := Load(write(t, `{"codex":{"enabled":true}}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Interval() != time.Minute {
		t.Fatalf("intervalo por defecto: %v", cfg.Interval())
	}
}

func TestIntervalFromConfig(t *testing.T) {
	cfg, err := Load(write(t, `{"codex":{"enabled":true},"interval_seconds":45}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Interval() != 45*time.Second {
		t.Fatalf("intervalo declarado: %v", cfg.Interval())
	}
}

func TestIntervalInvalid(t *testing.T) {
	for _, content := range invalidIntervals {
		if _, err := Load(write(t, content)); err == nil {
			t.Fatalf("se esperaba un error por interval_seconds inválido: %s", content)
		}
	}
}

var invalidIntervals = []string{
	`{"codex":{"enabled":true},"interval_seconds":0}`,
	`{"codex":{"enabled":true},"interval_seconds":-5}`,
	`{"codex":{"enabled":true},"interval_seconds":"60"}`,
}
