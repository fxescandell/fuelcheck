package providers_test

import (
	"github.com/fxescandell/fuelcheck/internal/providers"
	"testing"
)

func TestParseCodexUsage(t *testing.T) {
	result, err := providers.ParseCodexUsage([]byte(`{"rate_limit":{"primary_window":{"used_percent":0},"secondary_window":{"used_percent":26,"reset_at":1700000000}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Windows) != 2 {
		t.Fatalf("ventanas: %d", len(result.Windows))
	}
	if result.Windows[0].UsedPercent != 0 || result.Windows[0].ResetsAt != nil {
		t.Fatalf("ventana corta: %+v", result.Windows[0])
	}
	if result.Windows[1].UsedPercent != 26 || result.Windows[1].ResetsAt == nil || result.Windows[1].ResetsAt.Unix() != 1700000000 {
		t.Fatalf("ventana semanal: %+v", result.Windows[1])
	}
	if result.Windows[0].Duration != nil {
		t.Fatal("duración inventada")
	}
}

func TestParseCodexUsageInvalid(t *testing.T) {
	for _, input := range []string{`{`, `{}`, `{"rate_limit":42}`} {
		if _, err := providers.ParseCodexUsage([]byte(input)); err == nil {
			t.Errorf("aceptado payload inválido: %s", input)
		}
	}
}

func TestParseCodexUsageWithoutQuota(t *testing.T) {
	for _, input := range []string{`{"rate_limit":null}`, `{"rate_limit":{}}`} {
		result, err := providers.ParseCodexUsage([]byte(input))
		if err != nil || !result.OK || len(result.Windows) != 0 {
			t.Fatalf("respuesta sin cuota: %+v, %v", result, err)
		}
	}
}
