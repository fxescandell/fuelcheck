package providers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/fxescandell/fuelcheck/internal/auth"
)

const codexUsageURL = "https://chatgpt.com/backend-api/wham/usage"

// ParseCodexUsage transforma la respuesta de Codex en ventanas de uso.
func ParseCodexUsage(body []byte) (*ProviderResult, error) {
	var response struct {
		RateLimit json.RawMessage `json:"rate_limit"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("error al interpretar el JSON: %w", err)
	}
	if len(response.RateLimit) == 0 {
		return nil, fmt.Errorf("respuesta de Codex sin campo de cuota")
	}
	result := &ProviderResult{Provider: "Codex", OK: true}
	if string(response.RateLimit) == "null" {
		return result, nil
	}
	var limits struct {
		Primary *struct {
			UsedPercent *float64 `json:"used_percent"`
			ResetAt     *float64 `json:"reset_at"`
		} `json:"primary_window"`
		Secondary *struct {
			UsedPercent *float64 `json:"used_percent"`
			ResetAt     *float64 `json:"reset_at"`
		} `json:"secondary_window"`
	}
	if err := json.Unmarshal(response.RateLimit, &limits); err != nil {
		return nil, fmt.Errorf("error al interpretar las ventanas de Codex: %w", err)
	}
	add := func(kind string, used *float64, reset *float64) error {
		if used == nil || *used < 0 || *used > 100 {
			return fmt.Errorf("consumo inválido en ventana %s", kind)
		}
		w := UsageWindow{Kind: kind, UsedPercent: *used}
		if reset != nil && *reset > 0 {
			if *reset >= firstInvalidJSONUnixSecond {
				return fmt.Errorf("reinicio fuera del rango de fechas admitido")
			}
			t := time.Unix(int64(*reset), 0).UTC()
			w.ResetsAt = &t
		}
		result.Windows = append(result.Windows, w)
		return nil
	}
	if w := limits.Primary; w != nil {
		if err := add("corta", w.UsedPercent, w.ResetAt); err != nil {
			return nil, err
		}
	}
	if w := limits.Secondary; w != nil {
		if err := add("semanal", w.UsedPercent, w.ResetAt); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// FetchCodexUsage consulta la cuota usando las credenciales del usuario.
func FetchCodexUsage(configuredKey, endpoint string) *ProviderResult {
	fail := func(err error) *ProviderResult { return &ProviderResult{Provider: "Codex", Error: err.Error()} }
	creds, err := auth.GetCodexCredentials(configuredKey)
	if err != nil {
		return fail(err)
	}
	body, status, err := doCodexRequest(creds, endpoint)
	if err != nil {
		return fail(fmt.Errorf("error de conexión: %w", err))
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		if configuredKey != "" {
			return fail(fmt.Errorf("la API rechazó el token de Codex configurado; sustitúyelo en el fichero de configuración"))
		}
		if err := creds.RefreshToken(); err != nil {
			return fail(fmt.Errorf("el token caducó y no se pudo refrescar: %w", err))
		}
		body, status, err = doCodexRequest(creds, endpoint)
		if err != nil {
			return fail(fmt.Errorf("error de conexión tras refrescar el token: %w", err))
		}
	}
	if status != http.StatusOK {
		return fail(fmt.Errorf("la API respondió con estado %d", status))
	}
	result, err := ParseCodexUsage(body)
	if err != nil {
		return fail(err)
	}
	return result
}

func doCodexRequest(creds *auth.CodexCredentials, endpoint string) ([]byte, int, error) {
	req, err := http.NewRequest("GET", resolveURL(endpoint, codexUsageURL), nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "CodexBar")
	if creds.AccountID != "" {
		req.Header.Set("ChatGPT-Account-Id", creds.AccountID)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return body, resp.StatusCode, err
}
