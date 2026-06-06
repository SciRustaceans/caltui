// Package charts renders simple terminal charts: a single-line block sparkline
// (used on the dashboard) and multi-row ASCII line charts (used on the trends
// screen, via asciigraph).
package charts

import (
	"math"
	"strings"

	"github.com/guptarohit/asciigraph"
)

// LineChart renders values as a multi-row ASCII line chart with a left value
// axis. height is the number of plot rows; width caps the plot columns (0 = one
// column per point); precision is the number of decimals on the axis labels.
// Fewer than two points renders nothing.
func LineChart(values []float64, height, width int, precision uint) string {
	if len(values) < 2 {
		return ""
	}
	opts := []asciigraph.Option{asciigraph.Height(height), asciigraph.Precision(precision)}
	if width > 0 {
		opts = append(opts, asciigraph.Width(width))
	}
	return asciigraph.Plot(values, opts...)
}

// ProjectionChart plots an actual series (blue) and a projected continuation
// (coral) that begins where the actual data ends. The two are drawn as separate
// colored lines via NaN-gap padding. Needs at least two actual points.
func ProjectionChart(actual, projected []float64, height, width int, precision uint) string {
	if len(actual) < 2 {
		return ""
	}
	total := len(actual) + len(projected)
	hist := make([]float64, total)
	proj := make([]float64, total)
	for i := 0; i < total; i++ {
		hist[i] = math.NaN()
		proj[i] = math.NaN()
	}
	copy(hist, actual)
	// Anchor the projection to the last actual point so the lines connect.
	proj[len(actual)-1] = actual[len(actual)-1]
	for i, v := range projected {
		proj[len(actual)+i] = v
	}
	opts := []asciigraph.Option{
		asciigraph.Height(height),
		asciigraph.Precision(precision),
		asciigraph.SeriesColors(asciigraph.Blue, asciigraph.Coral),
	}
	if width > 0 {
		opts = append(opts, asciigraph.Width(width))
	}
	return asciigraph.PlotMany([][]float64{hist, proj}, opts...)
}

var sparkRunes = []rune("▁▂▃▄▅▆▇█")

// Sparkline renders values as a one-line block sparkline scaled between the min
// and max of the data. Empty input returns an empty string. When all values are
// equal it renders a flat mid-level line.
func Sparkline(values []float64) string {
	if len(values) == 0 {
		return ""
	}
	lo, hi := values[0], values[0]
	for _, v := range values {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	span := hi - lo
	var b strings.Builder
	for _, v := range values {
		idx := len(sparkRunes) / 2
		if span > 0 {
			idx = int((v-lo)/span*float64(len(sparkRunes)-1) + 0.5)
		}
		if idx < 0 {
			idx = 0
		}
		if idx > len(sparkRunes)-1 {
			idx = len(sparkRunes) - 1
		}
		b.WriteRune(sparkRunes[idx])
	}
	return b.String()
}
