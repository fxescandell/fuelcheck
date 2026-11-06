package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// MiniMaxCredentials contiene la clave de suscripción de MiniMax.
type MiniMaxCredentials struct {
	APIKey string
}

// GetMiniMaxCredentials lee la clave del fichero de configuración del
// cliente oficial de MiniMax (~/.mmx/config.json).
func GetMiniMaxCredentials(configuredKey string) (*MiniMaxCredentials, error) {
	if configuredKey != "" {
		return &MiniMaxCredentials{APIKey: configuredKey}, nil
	}
	path := home(".mmx/config.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer %s.\nInicia sesión con el CLI de MiniMax", path)
	}
	var cfg struct {
		APIKey string `json:"api_key"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("JSON inválido en %s", path)
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("%s no tiene api_key.\nInicia sesión con el CLI de MiniMax", path)
	}
	return &MiniMaxCredentials{APIKey: cfg.APIKey}, nil
}

// ZaiCredentials contiene la clave de API de z.ai.
type ZaiCredentials struct {
	APIKey string
}

var zaiKeyLine = regexp.MustCompile(`^\s*(?:export\s+)?ZAI_API_KEY\s*=\s*["']?([^"'\s]+)`)

// GetZaiCredentials encuentra la clave de z.ai en el entorno o en los
// ficheros de shell del usuario.
func GetZaiCredentials(configuredKey string) (*ZaiCredentials, error) {
	if configuredKey != "" {
		return &ZaiCredentials{APIKey: configuredKey}, nil
	}
	if key := os.Getenv("ZAI_API_KEY"); key != "" {
		return &ZaiCredentials{APIKey: key}, nil
	}
	for _, f := range []string{home(".zshrc"), home(".zprofile"), home(".bashrc"), home(".profile")} {
		raw, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(raw), "\n") {
			if m := zaiKeyLine.FindStringSubmatch(line); m != nil {
				return &ZaiCredentials{APIKey: m[1]}, nil
			}
		}
	}
	return nil, fmt.Errorf("no se encontró ZAI_API_KEY en el entorno ni en los ficheros de shell")
}

func home(rel string) string {
	h, err := os.UserHomeDir()
	if err != nil {
		return rel
	}
	return h + "/" + rel
}
