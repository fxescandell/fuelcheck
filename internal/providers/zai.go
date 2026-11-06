package providers

import (
	"encoding/json"
	"fmt"
	"time"
)

// ParseZaiUsage transforma la respuesta de z.ai en ventanas de uso.
//
// La API ya devuelve el porcentaje consumido. Codifica cada ventana con
// `unit`/`number`: 3/5 es la ventana de 5 horas y 6 la semanal. El pool
// mensual de herramientas (TIME_LIMIT) no es cuota del plan y se descarta.
func ParseZaiUsage(body []byte) (*ProviderResult, error) {
	var response struct {
		Data *struct {
			Limits []struct {
				Type          string   `json:"type"`
				Unit          int      `json:"unit"`
				Number        int      `json:"number"`
				Percentage    *float64 `json:"percentage"`
				NextResetTime int64    `json:"nextResetTime"`
			} `json:"limits"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("error al interpretar el JSON: %w", err)
	}
	if response.Data == nil {
		return nil, fmt.Errorf("respuesta de z.ai sin datos de cuota")
	}
	result := &ProviderResult{Provider: "z.ai", OK: true}
	byKind := map[string]UsageWindow{}
	has := map[string]bool{}
	for _, l := range response.Data.Limits {
		var kind string
		var dur time.Duration
		switch {
		case l.Type == "TIME_LIMIT":
			continue
		case l.Unit == 3 && l.Number == 5:
			// TOKENS_LIMIT y CREDIT_LIMIT comparten la semántica unit/number: la
			// ventana corta es exactamente el par 3/5.
			kind, dur = "corta", 5*time.Hour
		case l.Unit == 6:
			kind, dur = "semanal", 7*24*time.Hour
		default:
			continue
		}
		if l.Percentage == nil || *l.Percentage < 0 || *l.Percentage > 100 {
			return nil, fmt.Errorf("entrada de cuota %s/%d con porcentaje inválido", l.Type, l.Unit)
		}
		w := UsageWindow{Kind: kind, UsedPercent: *l.Percentage, Duration: &dur}
		if l.NextResetTime > 0 {
			if l.NextResetTime >= firstInvalidJSONUnixSecond*1000 {
				return nil, fmt.Errorf("reinicio fuera del rango de fechas admitido")
			}
			t := time.UnixMilli(l.NextResetTime).UTC()
			w.ResetsAt = &t
		}
		byKind[kind], has[kind] = worseWindow(byKind[kind], w, has[kind])
	}
	for _, kind := range []string{"corta", "semanal"} {
		if has[kind] {
			result.Windows = append(result.Windows, byKind[kind])
		}
	}
	return result, nil
}
