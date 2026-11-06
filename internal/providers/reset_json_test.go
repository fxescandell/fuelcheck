package providers_test

import (
	"testing"

	"github.com/fxescandell/fuelcheck/internal/providers"
)

func TestRejectsResetOutsideJSONTimeRange(t *testing.T) {
	tests := []struct {
		name  string
		parse func([]byte) (*providers.ProviderResult, error)
		body  string
	}{
		{"Codex", providers.ParseCodexUsage, `{"rate_limit":{"primary_window":{"used_percent":25,"reset_at":253402300800}}}`},
		{"MiniMax", providers.ParseMiniMaxUsage, `{"model_remains":[{"current_interval_remaining_percent":75,"end_time":253402300800000}]}`},
		{"z.ai", providers.ParseZaiUsage, `{"data":{"limits":[{"type":"TOKENS_LIMIT","unit":3,"number":5,"percentage":25,"nextResetTime":253402300800000}]}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.parse([]byte(tt.body)); err == nil {
				t.Fatal("reinicio fuera del rango JSON aceptado")
			}
		})
	}
}
