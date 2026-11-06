package ui_test

import (
	"github.com/fxescandell/fuelcheck/internal/providers"
	"github.com/fxescandell/fuelcheck/internal/ui"
	"strings"
	"testing"
	"time"
)

func TestAutomaticLayoutFitsTerminal(t *testing.T) {
	results := []*providers.ProviderResult{
		{Provider: "Codex", OK: true, Windows: []providers.UsageWindow{{Kind: "corta", UsedPercent: 50}, {Kind: "semanal", UsedPercent: 75}}},
		{Provider: "MiniMax", OK: true, Windows: []providers.UsageWindow{{Kind: "corta", UsedPercent: 50}, {Kind: "semanal", UsedPercent: 75}}},
		{Provider: "z.ai", OK: true, Windows: []providers.UsageWindow{{Kind: "corta", UsedPercent: 50}, {Kind: "semanal", UsedPercent: 75}}},
	}
	if got := ui.RenderLayout(results, time.Time{}, false, 44, "auto"); strings.Count(got, "\n") != 2 {
		t.Fatalf("44 columnas debe ser vertical: %q", got)
	}
	if got := ui.RenderLayout(results, time.Time{}, false, 100, "auto"); strings.Contains(got, "\n") {
		t.Fatalf("100 columnas debe ser horizontal: %q", got)
	}
	if got := ui.RenderLayout(results, time.Time{}, false, 100, "vertical"); strings.Count(got, "\n") != 2 {
		t.Fatalf("vertical forzado: %q", got)
	}
	if got := ui.RenderLayout(results, time.Time{}, false, 44, "horizontal"); strings.Contains(got, "\n") {
		t.Fatalf("horizontal forzado: %q", got)
	}
}

func TestVerticalTruncatesNameAndShowsStates(t *testing.T) {
	rows := []*providers.ProviderResult{
		{Provider: "ProveedorExtraordinariamenteLargo", OK: true, Windows: []providers.UsageWindow{{Kind: "corta", UsedPercent: 10}}},
		{Provider: "Fallido", Error: "sin credenciales"},
		{Provider: "Vacio", OK: true},
	}
	got := ui.RenderLayout(rows, time.Time{}, false, 20, "vertical")
	want := "● Proveedor… 10% — ●\n● Fallido error: si…\n● Vacio sin cuota"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestVerticalAlignsProviders(t *testing.T) {
	rows := []*providers.ProviderResult{
		{Provider: "Codex", OK: true, Windows: []providers.UsageWindow{{Kind: "corta", UsedPercent: 10}}},
		{Provider: "MiniMax", OK: true, Windows: []providers.UsageWindow{{Kind: "corta", UsedPercent: 20}}},
	}
	got := ui.RenderLayout(rows, time.Time{}, false, 44, "vertical")
	want := "● Codex   10% — ●\n● MiniMax 20% — ●"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
