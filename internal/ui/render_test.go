package ui_test

import (
	"github.com/fxescandell/fuelcheck/internal/providers"
	"github.com/fxescandell/fuelcheck/internal/ui"
	"strings"
	"testing"
	"time"
)

func TestRenderCodexWindows(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	short := now.Add(116 * time.Minute)
	weekly := now.Add(6*24*time.Hour + 16*time.Hour)
	results := []*providers.ProviderResult{{Provider: "Codex", OK: true, Windows: []providers.UsageWindow{{Kind: "corta", UsedPercent: 100, ResetsAt: &short}, {Kind: "semanal", UsedPercent: 26, ResetsAt: &weekly}}}}
	got := ui.Render(results, now, true)
	want := "\x1b[31m●\x1b[0m \x1b[31mCodex\x1b[0m 100% 1h 56m 26% 6d 16h \x1b[31m●\x1b[0m\x1b[32m●\x1b[0m"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRenderSeparatesProvidersAndMissingReset(t *testing.T) {
	results := []*providers.ProviderResult{{Provider: "Codex", OK: true, Windows: []providers.UsageWindow{{Kind: "corta", UsedPercent: 30}}}, {Provider: "MiniMax", OK: true, Windows: []providers.UsageWindow{{Kind: "corta", UsedPercent: 65}}}}
	got := ui.Render(results, time.Time{}, false)
	want := "● Codex   30% — ● │ ● MiniMax 65% — ●"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRenderColorThresholds(t *testing.T) {
	for _, tc := range []struct {
		used  float64
		color string
	}{
		{30, "32"},
		{30.1, "33"},
		{65, "33"},
		{65.1, "31"},
	} {
		results := []*providers.ProviderResult{{Provider: "Codex", OK: true, Windows: []providers.UsageWindow{{Kind: "corta", UsedPercent: tc.used}}}}
		got := ui.Render(results, time.Time{}, true)
		if !strings.HasPrefix(got, "\x1b["+tc.color+"m●\x1b[0m") {
			t.Errorf("consumo %v: color incorrecto: %q", tc.used, got)
		}
	}
}

func TestRenderErrorStaysOnOneLine(t *testing.T) {
	results := []*providers.ProviderResult{{Provider: "Codex", Error: "sin credenciales\nInicia sesión"}}
	if got := ui.Render(results, time.Time{}, false); got != "● Codex error: sin credenciales Inicia sesión" {
		t.Fatalf("error inesperado: %q", got)
	}
}

func TestRenderGraySemaphoreStates(t *testing.T) {
	gray := "\x1b[90m●\x1b[0m \x1b[90m"
	for _, tc := range []struct {
		name    string
		result  providers.ProviderResult
		address string
	}{
		{"error", providers.ProviderResult{Provider: "Codex", Error: "sin credenciales"}, " error: sin credenciales"},
		{"sin cuota", providers.ProviderResult{Provider: "Codex", OK: true}, " sin cuota"},
		{"ilimitado", providers.ProviderResult{Provider: "Codex", OK: true, Windows: []providers.UsageWindow{{Kind: "corta", Unlimited: true}}}, " ilimitado \x1b[90m●\x1b[0m"},
	} {
		results := []*providers.ProviderResult{&tc.result}
		got := ui.Render(results, time.Time{}, true)
		if !strings.HasPrefix(got, gray+"Codex\x1b[0m"+tc.address) {
			t.Errorf("%s: semáforo o texto incorrecto: %q", tc.name, got)
		}
	}
}
