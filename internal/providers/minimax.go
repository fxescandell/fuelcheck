package providers

import (
	"encoding/json"
	"fmt"
	"time"
)

// ParseMiniMaxUsage transforma la respuesta de MiniMax en ventanas de uso.
//
// La API devuelve el porcentaje restante y una entrada por modelo; aquí se
// normaliza a consumido y se conserva por tipo canónico la ventana finita
// con mayor consumo, junto con su propio reinicio y duración.
func ParseMiniMaxUsage(body []byte) (*ProviderResult, error) {
	var response struct {
		ModelRemains json.RawMessage `json:"model_remains"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("error al interpretar el JSON: %w", err)
	}
	if len(response.ModelRemains) == 0 {
		return nil, fmt.Errorf("respuesta de MiniMax sin campo de modelos")
	}
	result := &ProviderResult{Provider: "MiniMax", OK: true}
	if string(response.ModelRemains) == "null" {
		return result, nil
	}
	var models []struct {
		ModelName                string   `json:"model_name"`
		StartTime                int64    `json:"start_time"`
		EndTime                  int64    `json:"end_time"`
		CurrentIntervalRemaining *float64 `json:"current_interval_remaining_percent"`
		CurrentIntervalStatus    *int     `json:"current_interval_status"`
		WeeklyStartTime          int64    `json:"weekly_start_time"`
		WeeklyEndTime            int64    `json:"weekly_end_time"`
		CurrentWeeklyRemaining   *float64 `json:"current_weekly_remaining_percent"`
		CurrentWeeklyStatus      *int     `json:"current_weekly_status"`
	}
	if err := json.Unmarshal(response.ModelRemains, &models); err != nil {
		return nil, fmt.Errorf("error al interpretar las ventanas de MiniMax: %w", err)
	}
	byKind := map[string]UsageWindow{}
	has := map[string]bool{}
	for _, m := range models {
		if m.CurrentIntervalRemaining == nil && m.CurrentWeeklyRemaining == nil {
			return nil, fmt.Errorf("modelo %q sin porcentajes de cuota", m.ModelName)
		}
		if m.CurrentIntervalRemaining != nil {
			w := UsageWindow{Kind: "corta", UsedPercent: 100 - *m.CurrentIntervalRemaining, Unlimited: m.CurrentIntervalStatus != nil && *m.CurrentIntervalStatus == 3}
			if m.EndTime > 0 {
				if m.EndTime >= firstInvalidJSONUnixSecond*1000 {
					return nil, fmt.Errorf("reinicio fuera del rango de fechas admitido")
				}
				t := time.UnixMilli(m.EndTime).UTC()
				w.ResetsAt = &t
			}
			if m.EndTime > m.StartTime {
				d := time.UnixMilli(m.EndTime).Sub(time.UnixMilli(m.StartTime))
				w.Duration = &d
			}
			byKind["corta"], has["corta"] = worseWindow(byKind["corta"], w, has["corta"])
		}
		if m.CurrentWeeklyRemaining != nil {
			w := UsageWindow{Kind: "semanal", UsedPercent: 100 - *m.CurrentWeeklyRemaining, Unlimited: m.CurrentWeeklyStatus != nil && *m.CurrentWeeklyStatus == 3}
			if m.WeeklyEndTime > 0 {
				if m.WeeklyEndTime >= firstInvalidJSONUnixSecond*1000 {
					return nil, fmt.Errorf("reinicio fuera del rango de fechas admitido")
				}
				t := time.UnixMilli(m.WeeklyEndTime).UTC()
				w.ResetsAt = &t
			}
			if m.WeeklyEndTime > m.WeeklyStartTime {
				d := time.UnixMilli(m.WeeklyEndTime).Sub(time.UnixMilli(m.WeeklyStartTime))
				w.Duration = &d
			}
			byKind["semanal"], has["semanal"] = worseWindow(byKind["semanal"], w, has["semanal"])
		}
	}
	for _, kind := range []string{"corta", "semanal"} {
		if has[kind] {
			result.Windows = append(result.Windows, byKind[kind])
		}
	}
	return result, nil
}

// worseWindow conserva juntos el porcentaje, reinicio y duración de una cuota.
// En caso de empate entre consumos se elige el reinicio más próximo.
func worseWindow(a, b UsageWindow, hasA bool) (UsageWindow, bool) {
	if !hasA {
		return b, true
	}
	if b.Unlimited && !a.Unlimited {
		return a, true
	}
	if a.Unlimited && !b.Unlimited {
		return b, true
	}
	if b.UsedPercent > a.UsedPercent {
		return b, true
	}
	if b.UsedPercent == a.UsedPercent && b.ResetsAt != nil && (a.ResetsAt == nil || b.ResetsAt.Before(*a.ResetsAt)) {
		return b, true
	}
	return a, true
}
