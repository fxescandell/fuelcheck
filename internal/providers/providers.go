package providers

import (
	"fmt"
	"io"
	"net/http"

	"github.com/fxescandell/fuelcheck/internal/auth"
)

const (
	minimaxUsageURL = "https://api.minimax.io/v1/token_plan/remains"
	zaiUsageURL     = "https://api.z.ai/api/monitor/usage/quota/limit"
)

// resolveURL prefiere el endpoint declarado por el usuario; vacío, la URL oficial.
func resolveURL(endpoint, fallback string) string {
	if endpoint != "" {
		return endpoint
	}
	return fallback
}

// FetchMiniMaxUsage consulta la cuota de la suscripción de MiniMax.
func FetchMiniMaxUsage(configuredKey, endpoint string) *ProviderResult {
	fail := func(err error) *ProviderResult { return &ProviderResult{Provider: "MiniMax", Error: err.Error()} }
	creds, err := auth.GetMiniMaxCredentials(configuredKey)
	if err != nil {
		return fail(err)
	}
	body, status, err := getJSON(resolveURL(endpoint, minimaxUsageURL), map[string]string{
		"Authorization": "Bearer " + creds.APIKey,
	})
	if err != nil {
		return fail(fmt.Errorf("error de conexión: %w", err))
	}
	if status != 200 {
		return fail(fmt.Errorf("la API respondió con estado %d", status))
	}
	result, err := ParseMiniMaxUsage(body)
	if err != nil {
		return fail(err)
	}
	return result
}

// FetchZaiUsage consulta la cuota del plan de z.ai.
func FetchZaiUsage(configuredKey, endpoint string) *ProviderResult {
	fail := func(err error) *ProviderResult { return &ProviderResult{Provider: "z.ai", Error: err.Error()} }
	creds, err := auth.GetZaiCredentials(configuredKey)
	if err != nil {
		return fail(err)
	}
	// La clave va cruda: z.ai no admite el prefijo Bearer en este endpoint.
	body, status, err := getJSON(resolveURL(endpoint, zaiUsageURL), map[string]string{
		"Authorization": creds.APIKey,
	})
	if err != nil {
		return fail(fmt.Errorf("error de conexión: %w", err))
	}
	if status != 200 {
		return fail(fmt.Errorf("la API respondió con estado %d", status))
	}
	result, err := ParseZaiUsage(body)
	if err != nil {
		return fail(err)
	}
	return result
}

func getJSON(url string, headers map[string]string) ([]byte, int, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return body, resp.StatusCode, err
}
