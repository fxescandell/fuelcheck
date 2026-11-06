package ui

import (
	"strings"
	"time"

	"github.com/fxescandell/fuelcheck/internal/providers"
	"github.com/rivo/uniseg"
)

// RenderLayout elige el formato según el ancho disponible; ancho cero indica
// que no se conoce el tamaño del terminal y conserva la salida horizontal.
func RenderLayout(results []*providers.ProviderResult, now time.Time, color bool, width int, layout string) string {
	horizontal := Render(results, now, color)
	if layout == "horizontal" {
		return horizontal
	}
	if layout != "vertical" {
		if width <= 0 {
			return horizontal
		}
		plain := horizontal
		if color {
			plain = Render(results, now, false)
		}
		if uniseg.StringWidth(plain) <= width {
			return horizontal
		}
	}
	displayed := make([]providers.ProviderResult, len(results))
	maxName := 0
	for i, result := range results {
		displayed[i] = *result
		row := &displayed[i]
		if width > 0 {
			if !row.OK {
				// Reservar espacio para el nombre y el prefijo de error.
				row.Error = strings.Join(strings.Fields(row.Error), " ")
				base := *row
				base.Provider, base.Error = "", ""
				nameBudget := uniseg.StringWidth(row.Provider)
				if nameBudget > width/2 {
					nameBudget = width / 2
				}
				budget := width - uniseg.StringWidth(Render([]*providers.ProviderResult{&base}, now, false)) - nameBudget
				if budget < 0 {
					budget = 0
				}
				row.Error = truncateName(row.Error, budget)
			}
			base := *row
			base.Provider = ""
			available := width - uniseg.StringWidth(Render([]*providers.ProviderResult{&base}, now, false))
			if available > 0 {
				row.Provider = truncateName(row.Provider, available)
			} else {
				row.Provider = truncateName(row.Provider, 1)
			}
		}
		if n := uniseg.StringWidth(row.Provider); n > maxName {
			maxName = n
		}
	}
	rows := make([]string, 0, len(displayed))
	for i := range displayed {
		row := &displayed[i]
		padding := maxName - uniseg.StringWidth(row.Provider)
		plainWidth := uniseg.StringWidth(Render([]*providers.ProviderResult{row}, now, false))
		if width > 0 && plainWidth+padding > width {
			padding = 0
		}
		row.Provider += strings.Repeat(" ", padding)
		rows = append(rows, Render([]*providers.ProviderResult{row}, now, color))
	}
	return strings.Join(rows, "\n")
}

func truncateName(name string, available int) string {
	if available <= 0 {
		return ""
	}
	if uniseg.StringWidth(name) <= available {
		return name
	}
	if available == 1 {
		return "…"
	}
	var b strings.Builder
	g := uniseg.NewGraphemes(name)
	remaining := available - 1
	for g.Next() {
		if g.Width() > remaining {
			break
		}
		b.WriteString(g.Str())
		remaining -= g.Width()
	}
	b.WriteString("…")
	return b.String()
}
