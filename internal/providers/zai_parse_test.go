package providers_test

import (
	"strings"
	"testing"
	"time"

	"github.com/fxescandell/fuelcheck/internal/providers"
	"github.com/fxescandell/fuelcheck/internal/ui"
)

func TestParseZaiUsage(t *testing.T) {
	payload := `{
  "code": 200, "msg": "Operation successful", "success": true,
  "data": {"level": "lite", "limits": [
   {"type": "TIME_LIMIT", "unit": 5, "number": 1, "percentage": 38, "nextResetTime": 1794310692975},
   {"type": "TOKENS_LIMIT", "unit": 3, "number": 5, "percentage": 4, "nextResetTime": 1793986237090},
   {"type": "CREDIT_LIMIT", "unit": 3, "number": 5, "percentage": 6, "nextResetTime": 1793986237090},
   {"type": "TOKENS_LIMIT", "unit": 6, "percentage": 70, "nextResetTime": 1794182400000}
  ]}
 }`
	result, err := providers.ParseZaiUsage([]byte(payload))
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Provider != "z.ai" {
		t.Fatalf("resultado: %+v", result)
	}
	if len(result.Windows) != 2 {
		t.Fatalf("ventanas: %d, quiere 2 (tokens y créditos colapsan; mensual descartada)", len(result.Windows))
	}
	short := result.Windows[0]
	if short.Kind != "corta" || short.UsedPercent != 6 {
		t.Fatalf("corta: %+v (tokens y créditos colapsan al peor consumo)", short)
	}
	if short.Duration == nil || *short.Duration != 5*time.Hour {
		t.Fatalf("duración corta: %v", short.Duration)
	}
	weekly := result.Windows[1]
	if weekly.Kind != "semanal" || weekly.UsedPercent != 70 {
		t.Fatalf("semanal: %+v", weekly)
	}
	if weekly.Duration == nil || *weekly.Duration != 7*24*time.Hour {
		t.Fatalf("duración semanal: %v", weekly.Duration)
	}
	if short.ResetsAt == nil || short.ResetsAt.UnixMilli() != 1793986237090 {
		t.Fatalf("reinicio en milisegundos: %+v", short.ResetsAt)
	}
}

func TestParseZaiCollapseKeepsWorstWindowReset(t *testing.T) {
	payload := `{"data":{"limits":[
	  {"type":"TOKENS_LIMIT","unit":3,"number":5,"percentage":20,"nextResetTime":1793986237090},
	  {"type":"CREDIT_LIMIT","unit":3,"number":5,"percentage":80,"nextResetTime":1793993437090}
	]}}`
	result, err := providers.ParseZaiUsage([]byte(payload))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Windows) != 1 {
		t.Fatalf("ventanas: %d", len(result.Windows))
	}
	w := result.Windows[0]
	if w.UsedPercent != 80 || w.ResetsAt == nil || w.ResetsAt.UnixMilli() != 1793993437090 {
		t.Fatalf("el reinicio debe pertenecer a la cuota con mayor consumo: %+v", w)
	}
}

func TestParseZaiUsageInvalid(t *testing.T) {
	for _, input := range []string{`{`, `{}`, `{"data":42}`} {
		if _, err := providers.ParseZaiUsage([]byte(input)); err == nil {
			t.Errorf("aceptado payload inválido: %s", input)
		}
	}
}

func TestParseZaiUsageOnlyMonthlyTools(t *testing.T) {
	payload := `{"data":{"limits":[{"type":"TIME_LIMIT","unit":5,"number":1,"percentage":38,"nextResetTime":1794310692975}]}}`
	result, err := providers.ParseZaiUsage([]byte(payload))
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatal("debería ser OK")
	}
	if len(result.Windows) != 0 {
		t.Fatalf("el pool mensual se descarta: %+v", result.Windows)
	}
}

func TestParseZaiUsagePastReset(t *testing.T) {
	payload := `{"data":{"limits":[{"type":"TOKENS_LIMIT","unit":3,"number":5,"percentage":10,"nextResetTime":1000}]}}`
	result, err := providers.ParseZaiUsage([]byte(payload))
	if err != nil {
		t.Fatal(err)
	}
	if result.Windows[0].ResetsAt == nil || result.Windows[0].ResetsAt.UnixMilli() != 1000 {
		t.Fatalf("reinicio pasado: %+v", result.Windows[0].ResetsAt)
	}
}

func TestParseZaiUsageCreditWithoutPairFive(t *testing.T) {
	// CREDIT_LIMIT con unit 3 pero number 1 no es la ventana corta: se descarta.
	payload := `{"data":{"limits":[{"type":"CREDIT_LIMIT","unit":3,"number":1,"percentage":25,"nextResetTime":1794310692975}]}}`
	result, err := providers.ParseZaiUsage([]byte(payload))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Windows) != 0 {
		t.Fatalf("el par 3/1 no es cuota del plan: %+v", result.Windows)
	}
}

func TestParseZaiUsageEntryWithoutPercentage(t *testing.T) {
	payload := `{"data":{"limits":[{"type":"TOKENS_LIMIT","unit":3,"number":5,"nextResetTime":1793986237090}]}}`
	if _, err := providers.ParseZaiUsage([]byte(payload)); err == nil {
		t.Fatal("una entrada de cuota sin porcentaje debería ser error")
	}
}

func TestParseZaiUsageWithoutWeeklyRendersOneRow(t *testing.T) {
	// Captura real de hoy: tokens y créditos colapsados, sin semanal.
	payload := `{"data":{"level":"lite","limits":[
	 {"type":"TIME_LIMIT","unit":5,"number":1,"percentage":38,"nextResetTime":1794310692975},
	 {"type":"TOKENS_LIMIT","unit":3,"number":5,"percentage":4,"nextResetTime":1793986237090},
	 {"type":"CREDIT_LIMIT","unit":3,"number":5,"percentage":6,"nextResetTime":1793986237090}
	]}}`
	result, err := providers.ParseZaiUsage([]byte(payload))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Windows) != 1 || result.Windows[0].Kind != "corta" {
		t.Fatalf("una sola ventana corta esperada: %+v", result.Windows)
	}
	line := ui.Render([]*providers.ProviderResult{result}, time.UnixMilli(1793980000000), false)
	if strings.Count(line, "│") != 0 {
		t.Fatalf("un solo proveedor, sin separadores: %q", line)
	}
	if strings.Contains(line, "38%") {
		t.Fatalf("el pool mensual no debe aparecer: %q", line)
	}
	if !strings.Contains(line, "z.ai") || strings.Count(line, "●") != 2 {
		t.Fatalf("fila sin huecos con una sola bolita extra: %q", line)
	}
}
