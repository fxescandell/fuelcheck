package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/fxescandell/fuelcheck/internal/auth"
	"github.com/fxescandell/fuelcheck/internal/config"
	"github.com/fxescandell/fuelcheck/internal/providers"
	"github.com/fxescandell/fuelcheck/internal/ui"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// Version se fija en tiempo de compilación con -ldflags.
var Version = "dev"

// Los proveedores disponibles.
var allProviders = map[string]func(string, string) *providers.ProviderResult{
	"codex":   providers.FetchCodexUsage,
	"minimax": providers.FetchMiniMaxUsage,
	"z.ai":    providers.FetchZaiUsage,
}

var providerOrder = []string{"codex", "minimax", "z.ai"}

var jsonMode bool
var layout string
var watch bool
var interval time.Duration

func main() {
	rootCmd := &cobra.Command{
		Use:   "ai-usage [providers...]",
		Short: "El estado de tus cuotas de AI coding plans en una ventana pequeña de terminal",
		Long: "Consulta el estado de uso de tus cuotas de AI coding plans.\n\n" +
			"Proveedores disponibles: " + strings.Join(providerOrder, ", ") + "\n\n" +
			"Ejemplos:\n" +
			"  ai-usage           # todos los proveedores configurados\n" +
			"  ai-usage codex     # solo Codex\n" +
			"  ai-usage --json    # salida JSON",
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: providerCompletion,
		RunE:              run,
		SilenceUsage:      true,
		Version:           Version,
	}

	rootCmd.Flags().BoolVar(&jsonMode, "json", false, "salida en formato JSON")
	rootCmd.Flags().StringVar(&layout, "layout", "auto", "diseño: auto, horizontal o vertical")
	rootCmd.Flags().BoolVar(&watch, "watch", false, "actualiza el widget hasta pulsar Ctrl-C")
	rootCmd.Flags().DurationVar(&interval, "interval", time.Minute, "intervalo entre actualizaciones (ejemplo: 30s o 2m)")
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func providerCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var suggestions []string
	for _, p := range providerOrder {
		if !contains(args, p) {
			suggestions = append(suggestions, p)
		}
	}
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

func run(cmd *cobra.Command, args []string) error {
	if layout != "auto" && layout != "horizontal" && layout != "vertical" {
		return fmt.Errorf("diseño desconocido: %q (usa auto, horizontal o vertical)", layout)
	}
	if watch && interval <= 0 {
		return fmt.Errorf("el intervalo debe ser mayor que cero")
	}
	path, err := config.Path()
	if err != nil {
		return err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	warnUndeclared(cmd, cfg, path)

	var selected []string
	for _, name := range providerOrder {
		if cfg.Provider(name).Enabled {
			selected = append(selected, name)
		}
	}
	if len(args) > 0 {
		selected = nil
		for _, arg := range args {
			name := strings.ToLower(strings.TrimSpace(arg))
			if _, ok := allProviders[name]; !ok {
				return fmt.Errorf("proveedor desconocido: %q\nDisponibles: %s",
					arg, strings.Join(providerOrder, ", "))
			}
			if !cfg.Provider(name).Enabled {
				return fmt.Errorf("el proveedor %s no está activo en %s", name, path)
			}
			selected = append(selected, name)
		}
	}

	if len(selected) == 0 {
		return fmt.Errorf("no hay proveedores activos en %s", path)
	}
	// Sin flag, --watch sigue el intervalo declarado en el fichero de configuración.
	if watch && !cmd.Flags().Changed("interval") {
		interval = cfg.Interval()
	}
	if watch {
		return runWatch(selected, cfg)
	}
	results := fetchSelected(selected, cfg)

	if jsonMode {
		if err := ui.WriteJSON(os.Stdout, results); err != nil {
			return fmt.Errorf("error al escribir JSON: %w", err)
		}
	} else {
		color := os.Getenv("NO_COLOR") == "" && termenv.NewOutput(os.Stdout).ColorProfile() != termenv.Ascii
		width, _, _ := term.GetSize(int(os.Stdout.Fd()))
		fmt.Println(ui.RenderLayout(results, time.Now(), color, width, layout))
	}

	for _, r := range results {
		if r.OK {
			return nil
		}
	}
	return fmt.Errorf("ningún proveedor devolvió datos")
}

func runWatch(selected []string, cfg config.Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	terminal := term.IsTerminal(int(os.Stdout.Fd())) && os.Getenv("TERM") != "dumb" && !jsonMode
	if terminal {
		if _, err := fmt.Fprint(os.Stdout, "\x1b[?1049h"); err != nil {
			return fmt.Errorf("error al escribir en la terminal: %w", err)
		}
		defer fmt.Fprint(os.Stdout, "\x1b[?1049l")
	}

	var statusErr error
	for {
		// Esperar la consulta evita interrumpir una renovación de credenciales
		// mientras Codex escribe su token actualizado.
		current := fetchSelected(selected, cfg)
		if ctx.Err() != nil {
			return statusErr
		}
		statusErr = fmt.Errorf("ningún proveedor devolvió datos")
		for _, result := range current {
			if result.OK {
				statusErr = nil
				break
			}
		}

		if jsonMode {
			if err := ui.WriteJSON(os.Stdout, current); err != nil {
				return fmt.Errorf("error al escribir JSON: %w", err)
			}
		} else {
			if terminal {
				if _, err := fmt.Fprint(os.Stdout, "\x1b[H\x1b[2J"); err != nil {
					return fmt.Errorf("error al escribir en la terminal: %w", err)
				}
			}
			color := os.Getenv("NO_COLOR") == "" && termenv.NewOutput(os.Stdout).ColorProfile() != termenv.Ascii
			width, _, _ := term.GetSize(int(os.Stdout.Fd()))
			if _, err := fmt.Fprintln(os.Stdout, ui.RenderLayout(current, time.Now(), color, width, layout)); err != nil {
				return fmt.Errorf("error al escribir el widget: %w", err)
			}
		}

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return statusErr
		case <-timer.C:
		}
	}
}

func fetchSelected(names []string, cfg config.Config) []*providers.ProviderResult {
	type indexedResult struct {
		index  int
		result *providers.ProviderResult
	}

	var wg sync.WaitGroup
	ch := make(chan indexedResult, len(names))

	for i, name := range names {
		fn := allProviders[name]
		provider := cfg.Provider(name)
		wg.Add(1)
		go func(idx int, key string, endpoint string, fetch func(string, string) *providers.ProviderResult) {
			defer wg.Done()
			ch <- indexedResult{index: idx, result: fetch(key, endpoint)}
		}(i, provider.APIKey, provider.Endpoint, fn)
	}

	wg.Wait()
	close(ch)

	results := make([]*providers.ProviderResult, len(names))
	for ir := range ch {
		results[ir.index] = ir.result
	}

	return results
}

func warnUndeclared(cmd *cobra.Command, cfg config.Config, path string) {
	for _, name := range providerOrder {
		if _, declared := cfg.Providers[name]; declared {
			continue
		}
		var err error
		switch name {
		case "codex":
			_, err = auth.GetCodexCredentials("")
		case "minimax":
			_, err = auth.GetMiniMaxCredentials("")
		case "z.ai":
			_, err = auth.GetZaiCredentials("")
		}
		if err == nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Proveedor %s detectado pero no declarado; añade %q: {\"enabled\": true} a %s.\n", name, name, path)
		}
	}
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
