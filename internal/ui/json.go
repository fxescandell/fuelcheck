package ui

import (
	"encoding/json"
	"io"

	"github.com/fxescandell/fuelcheck/internal/providers"
)

// WriteJSON escribe un objeto por proveedor con su estado y ventanas normalizadas.
func WriteJSON(w io.Writer, results []*providers.ProviderResult) error {
	type providerJSON struct {
		OK      bool                    `json:"ok"`
		Error   string                  `json:"error,omitempty"`
		Windows []providers.UsageWindow `json:"windows"`
	}
	output := make(map[string]providerJSON, len(results))
	for _, result := range results {
		windows := result.Windows
		if windows == nil {
			windows = []providers.UsageWindow{}
		}
		var message string
		if !result.OK {
			// Los errores pueden contener datos arbitrarios de respuestas o credenciales.
			message = "no se pudo consultar el proveedor"
		}
		output[result.Provider] = providerJSON{OK: result.OK, Error: message, Windows: windows}
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}
