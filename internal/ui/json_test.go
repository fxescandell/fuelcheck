package ui_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fxescandell/fuelcheck/internal/providers"
	"github.com/fxescandell/fuelcheck/internal/ui"
)

func TestJSONShowsNormalizedStateWithoutRawPayload(t *testing.T) {
	codex, err := providers.ParseCodexUsage([]byte(`{"secret":"should-not-appear","rate_limit":{"primary_window":{"used_percent":25,"reset_at":1767323045}}}`))
	if err != nil {
		t.Fatal(err)
	}
	results := []*providers.ProviderResult{
		codex,
		{Provider: "MiniMax", OK: true},
		{Provider: "z.ai", Error: "secret-credential"},
	}
	var output bytes.Buffer
	if err := ui.WriteJSON(&output, results); err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"Codex\": {\n    \"ok\": true,\n    \"windows\": [\n      {\n        \"kind\": \"corta\",\n        \"used_percent\": 25,\n        \"resets_at\": \"2026-01-02T03:04:05Z\"\n      }\n    ]\n  },\n  \"MiniMax\": {\n    \"ok\": true,\n    \"windows\": []\n  },\n  \"z.ai\": {\n    \"ok\": false,\n    \"error\": \"no se pudo consultar el proveedor\",\n    \"windows\": []\n  }\n}\n"
	if output.String() != want {
		t.Fatalf("salida inesperada:\n%s\nse esperaba:\n%s", output.String(), want)
	}
	if strings.Contains(output.String(), "should-not-appear") || strings.Contains(output.String(), "secret-credential") {
		t.Fatal("payload crudo o credencial expuesta")
	}
}
