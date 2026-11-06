package providers_test

import (
	"github.com/fxescandell/fuelcheck/internal/providers"
	"testing"
	"time"
)

func TestParseMiniMaxUsage(t *testing.T) {
	payload := `{
  "base_resp": {"status_code": 0, "status_msg": "success"},
  "model_remains": [
   {"model_name": "general", "start_time": 1793959200000, "end_time": 1793973600000,
    "current_interval_remaining_percent": 99, "current_interval_status": 1,
    "weekly_start_time": 1793577600000, "weekly_end_time": 1794182400000,
    "current_weekly_remaining_percent": 83, "current_weekly_status": 1},
   {"model_name": "video", "start_time": 1793923200000, "end_time": 1794009600000,
    "current_interval_remaining_percent": 100, "current_interval_status": 3,
    "weekly_start_time": 1793577600000, "weekly_end_time": 1794182400000,
    "current_weekly_remaining_percent": 100, "current_weekly_status": 3}
  ]
 }`
	result, err := providers.ParseMiniMaxUsage([]byte(payload))
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Provider != "MiniMax" {
		t.Fatalf("resultado: %+v", result)
	}
	if len(result.Windows) != 2 {
		t.Fatalf("ventanas: %d, quiere 2 (colapsadas por tipo)", len(result.Windows))
	}
	short := result.Windows[0]
	if short.Kind != "corta" {
		t.Fatalf("ventana corta esperada primero: %+v", short)
	}
	if short.UsedPercent != 1 {
		t.Fatalf("MiniMax da restante: quiere consumido 1, hay %v", short.UsedPercent)
	}
	if short.Duration == nil || *short.Duration != 4*time.Hour {
		t.Fatalf("duración corta: %v, quiere 4h (captura real de MiniMax)", short.Duration)
	}
	weekly := result.Windows[1]
	if weekly.Kind != "semanal" {
		t.Fatalf("ventana semanal: %+v", weekly)
	}
	if weekly.UsedPercent != 17 {
		t.Fatalf("semanal consumido: %v", weekly.UsedPercent)
	}
	if weekly.Duration == nil || *weekly.Duration != 7*24*time.Hour {
		t.Fatalf("duración semanal: %v", weekly.Duration)
	}
	if short.ResetsAt == nil || weekly.ResetsAt == nil {
		t.Fatal("faltan reinicios")
	}
}

func TestParseMiniMaxUsageInvalid(t *testing.T) {
	for _, input := range []string{`{`, `{}`, `{"model_remains":42}`} {
		if _, err := providers.ParseMiniMaxUsage([]byte(input)); err == nil {
			t.Errorf("aceptado payload inválido: %s", input)
		}
	}
}

func TestParseMiniMaxUsageUnlimited(t *testing.T) {
	result, err := providers.ParseMiniMaxUsage([]byte(`{"model_remains":[{"start_time":1,"end_time":2,"current_interval_remaining_percent":100,"current_interval_status":3,"current_weekly_remaining_percent":100,"current_weekly_status":3}]}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range result.Windows {
		if !w.Unlimited {
			t.Fatalf("ventana %s debería ser ilimitada", w.Kind)
		}
	}
}

func TestParseMiniMaxCollapseKeepsWorstWindowReset(t *testing.T) {
	// El reinicio mostrado debe pertenecer a la ventana con mayor consumo,
	// no a una cuota distinta que se reinicia antes.
	payload := `{"model_remains":[
	 {"model_name":"general","start_time":1793959200000,"end_time":1793973600000,
	  "current_interval_remaining_percent":89,"current_interval_status":1,
	  "weekly_start_time":1793577600000,"weekly_end_time":1794182400000,
	  "current_weekly_remaining_percent":50,"current_weekly_status":1},
	 {"model_name":"video","start_time":1793923200000,"end_time":1793937600000,
	  "current_interval_remaining_percent":95,"current_interval_status":1,
	  "weekly_start_time":1793577600000,"weekly_end_time":1794096000000,
	  "current_weekly_remaining_percent":60,"current_weekly_status":1}
	]}`
	result, err := providers.ParseMiniMaxUsage([]byte(payload))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Windows) != 2 {
		t.Fatalf("ventanas: %d", len(result.Windows))
	}
	short := result.Windows[0]
	if short.UsedPercent != 11 {
		t.Fatalf("peor consumo: %v, quiere 11", short.UsedPercent)
	}
	if short.ResetsAt == nil || short.ResetsAt.UnixMilli() != 1793973600000 {
		t.Fatalf("reinicio de la ventana corta de mayor consumo: %+v", short.ResetsAt)
	}
	weekly := result.Windows[1]
	if weekly.UsedPercent != 50 {
		t.Fatalf("peor consumo semanal: %v, quiere 50", weekly.UsedPercent)
	}
	if weekly.ResetsAt == nil || weekly.ResetsAt.UnixMilli() != 1794182400000 {
		t.Fatalf("reinicio de la ventana semanal de mayor consumo: %+v", weekly.ResetsAt)
	}
}

func TestParseMiniMaxCollapseKeepsWorstWindowDurationWhenLast(t *testing.T) {
	payload := `{"model_remains":[
	 {"model_name":"video","start_time":1793923200000,"end_time":1793926800000,
	  "current_interval_remaining_percent":95},
	 {"model_name":"general","start_time":1793959200000,"end_time":1793973600000,
	  "current_interval_remaining_percent":89}
	]}`
	result, err := providers.ParseMiniMaxUsage([]byte(payload))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Windows) != 1 {
		t.Fatalf("ventanas: %d", len(result.Windows))
	}
	w := result.Windows[0]
	if w.UsedPercent != 11 || w.ResetsAt == nil || w.ResetsAt.UnixMilli() != 1793973600000 || w.Duration == nil || *w.Duration != 4*time.Hour {
		t.Fatalf("la cuota y sus datos deben proceder del mismo modelo: %+v", w)
	}
}

func TestParseMiniMaxFiniteBeatsUnlimitedOnTie(t *testing.T) {
	// video (ilimitado) llega primero y empata con general (finito) al 100% restante.
	payload := `{"model_remains":[
	 {"model_name":"video","start_time":1793923200000,"end_time":1794009600000,
	  "current_interval_remaining_percent":100,"current_interval_status":3,
	  "weekly_start_time":1793577600000,"weekly_end_time":1794182400000,
	  "current_weekly_remaining_percent":100,"current_weekly_status":3},
	 {"model_name":"general","start_time":1793959200000,"end_time":1793973600000,
	  "current_interval_remaining_percent":100,"current_interval_status":1,
	  "weekly_start_time":1793577600000,"weekly_end_time":1794182400000,
	  "current_weekly_remaining_percent":100,"current_weekly_status":1}
	]}`
	result, err := providers.ParseMiniMaxUsage([]byte(payload))
	if err != nil {
		t.Fatal(err)
	}
	if result.Windows[0].Unlimited || result.Windows[1].Unlimited {
		t.Fatalf("la ventana finita debe ganar al empate: %+v", result.Windows)
	}
}

func TestParseMiniMaxModelWithoutPercentages(t *testing.T) {
	result, err := providers.ParseMiniMaxUsage([]byte(`{"model_remains":[{"model_name":"misterioso"}]}`))
	if err == nil {
		t.Fatal("un modelo sin porcentajes debería ser error, no éxito sin ventanas")
	}
	_ = result
}

func TestParseMiniMaxUsageWithoutQuota(t *testing.T) {
	result, err := providers.ParseMiniMaxUsage([]byte(`{"model_remains":[]}`))
	if err != nil || !result.OK || len(result.Windows) != 0 {
		t.Fatalf("sin cuota: %+v, %v", result, err)
	}
}
