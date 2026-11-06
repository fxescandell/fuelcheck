package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/fxescandell/fuelcheck/internal/providers"
	"github.com/rivo/uniseg"
)

// Render devuelve una sola línea; now y color permiten una presentación determinista.
func Render(results []*providers.ProviderResult, now time.Time, color bool) string {
	width := 0
	for _, r := range results {
		if n := uniseg.StringWidth(r.Provider); n > width {
			width = n
		}
	}
	parts := make([]string, 0, len(results))
	for _, r := range results {
		shade := 90
		if r.OK && len(r.Windows) > 0 {
			worst := 0.0
			hasFinite := false
			for _, w := range r.Windows {
				if !w.Unlimited {
					hasFinite = true
					if w.UsedPercent > worst {
						worst = w.UsedPercent
					}
				}
			}
			if hasFinite {
				shade = health(worst)
			}
		}
		part := paint("●", shade, color) + " " + paint(r.Provider, shade, color) + strings.Repeat(" ", width-uniseg.StringWidth(r.Provider))
		if !r.OK {
			parts = append(parts, part+" error: "+strings.Join(strings.Fields(r.Error), " "))
			continue
		}
		if len(r.Windows) == 0 {
			parts = append(parts, part+" sin cuota")
			continue
		}
		dots := ""
		for i := range r.Windows {
			index := i
			if len(r.Windows) == 2 && r.Windows[0].Kind == "semanal" && r.Windows[1].Kind == "corta" {
				index = 1 - i
			}
			w := r.Windows[index]
			if w.Unlimited {
				part += " ilimitado"
				dots += paint("●", 90, color)
				continue
			}
			part += fmt.Sprintf(" %d%% %s", int(math.Round(w.UsedPercent)), countdown(w.ResetsAt, now))
			dots += paint("●", health(w.UsedPercent), color)
		}
		parts = append(parts, part+" "+dots)
	}
	return strings.Join(parts, " │ ")
}

func health(used float64) int {
	switch {
	case used <= 30:
		return 32
	case used <= 65:
		return 33
	default:
		return 31
	}
}

func paint(s string, colorCode int, enabled bool) string {
	if !enabled {
		return s
	}
	return fmt.Sprintf("\x1b[%dm%s\x1b[0m", colorCode, s)
}

func countdown(reset *time.Time, now time.Time) string {
	if reset == nil {
		return "—"
	}
	d := reset.Sub(now)
	if d < 0 {
		d = 0
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh %02dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dd %02dh", int(d.Hours())/24, int(d.Hours())%24)
}
