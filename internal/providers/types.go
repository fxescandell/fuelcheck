package providers

import "time"

// El año 10000 no es representable como fecha RFC 3339 en JSON.
const firstInvalidJSONUnixSecond = 253402300800

// UsageWindow describe el consumo de una ventana de uso.
type UsageWindow struct {
	Kind        string         `json:"kind"`
	UsedPercent float64        `json:"used_percent"`
	ResetsAt    *time.Time     `json:"resets_at,omitempty"`
	Duration    *time.Duration `json:"duration,omitempty"`
	Unlimited   bool           `json:"unlimited,omitempty"`
}

// ProviderResult contiene el estado de un proveedor y sus ventanas.
type ProviderResult struct {
	Provider string        `json:"provider"`
	OK       bool          `json:"ok"`
	Error    string        `json:"error,omitempty"`
	Windows  []UsageWindow `json:"windows,omitempty"`
}
