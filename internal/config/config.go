// Package config carga los proveedores declarados por el usuario.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DefaultIntervalSeconds es el intervalo de sondeo cuando el fichero no declara uno.
const DefaultIntervalSeconds = 60

type Provider struct {
	Enabled bool   `json:"enabled"`
	APIKey  string `json:"api_key,omitempty"`
	// Endpoint sustituye a la URL oficial del proveedor cuando no está vacío.
	Endpoint string `json:"endpoint,omitempty"`
}

// Config es el contenido de config.json: una clave por proveedor más opciones
// generales como interval_seconds. El JSON no cambia de forma: las claves de
// proveedor siguen en la raíz del objeto.
type Config struct {
	Providers       map[string]Provider
	IntervalSeconds int // 0: sin declarar; se usa DefaultIntervalSeconds
}

// Provider devuelve la declaración de un proveedor (valor cero si no existe).
func (c Config) Provider(name string) Provider { return c.Providers[name] }

// Interval es el intervalo de sondeo efectivo.
func (c Config) Interval() time.Duration {
	if c.IntervalSeconds > 0 {
		return time.Duration(c.IntervalSeconds) * time.Second
	}
	return DefaultIntervalSeconds * time.Second
}

// Path devuelve la ruta XDG del fichero de configuración.
func Path() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("no se pudo localizar el directorio personal: %w", err)
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "ai-usage", "config.json"), nil
}

// Load lee una configuración sin consultar la red ni credenciales.
func Load(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("no se pudo leer la configuración %s: %w; crea el fichero para declarar tus proveedores", path, err)
	}
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("configuración JSON inválida en %s: %w", path, err)
	}
	return cfg, nil
}

// UnmarshalJSON acepta claves de proveedor y opciones generales en la raíz.
func (c *Config) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw == nil {
		return fmt.Errorf("la configuración debe ser un objeto JSON")
	}
	c.Providers = make(map[string]Provider, len(raw))
	for key, value := range raw {
		if key == "interval_seconds" {
			var seconds int
			if err := json.Unmarshal(value, &seconds); err != nil {
				return fmt.Errorf("interval_seconds debe ser un entero positivo: %w", err)
			}
			if seconds <= 0 || int64(seconds) > int64((1<<63-1)/time.Second) {
				return fmt.Errorf("interval_seconds fuera del rango admitido")
			}
			c.IntervalSeconds = seconds
			continue
		}
		if key != "codex" && key != "minimax" && key != "z.ai" {
			return fmt.Errorf("proveedor desconocido en la configuración: %q", key)
		}
		var provider Provider
		if err := json.Unmarshal(value, &provider); err != nil {
			return fmt.Errorf("proveedor %q: %w", key, err)
		}
		c.Providers[key] = provider
	}
	return nil
}
