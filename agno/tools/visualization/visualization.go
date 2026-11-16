// Package visualization ports Agno's VisualizationTools (bar, line, pie,
// scatter and histogram charts saved to an output directory).
//
// Deviation from Python: matplotlib PNGs are replaced by self-contained SVG
// files rendered in pure Go (no dependencies). A filename with another
// extension (e.g. "sales.png") is saved with the ".svg" extension. The JSON
// result shape (chart_type, title, file_path, data_points, status) matches Python.
package visualization

import (
	"encoding/json"
	"fmt"
	"html"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// VisualizationTools renders charts to SVG files.
type VisualizationTools struct {
	toolkit.Toolkit
	outputDir string

	enableBar, enableLine, enablePie, enableScatter, enableHistogram bool
}

// Option configures VisualizationTools.
type Option func(*VisualizationTools)

// WithOutputDir sets the output directory (default "charts").
func WithOutputDir(d string) Option { return func(v *VisualizationTools) { v.outputDir = d } }

// WithBarChart toggles CreateBarChart (default true).
func WithBarChart(b bool) Option { return func(v *VisualizationTools) { v.enableBar = b } }

// WithLineChart toggles CreateLineChart (default true).
func WithLineChart(b bool) Option { return func(v *VisualizationTools) { v.enableLine = b } }

// WithPieChart toggles CreatePieChart (default true).
func WithPieChart(b bool) Option { return func(v *VisualizationTools) { v.enablePie = b } }

// WithScatterPlot toggles CreateScatterPlot (default true).
func WithScatterPlot(b bool) Option { return func(v *VisualizationTools) { v.enableScatter = b } }

// WithHistogram toggles CreateHistogram (default true).
func WithHistogram(b bool) Option { return func(v *VisualizationTools) { v.enableHistogram = b } }

// BarChartParams for CreateBarChart.
type BarChartParams struct {
	Data     interface{} `json:"data" description:"Dictionary with categories as keys and numeric values, or list of dictionaries (label, value), or a JSON string of either." required:"true"`
	Title    string      `json:"title,omitempty" description:"Title of the chart (default \"Bar Chart\")"`
	XLabel   string      `json:"x_label,omitempty" description:"Label for x-axis (default \"Categories\")"`
	YLabel   string      `json:"y_label,omitempty" description:"Label for y-axis (default \"Values\")"`
	Filename string      `json:"filename,omitempty" description:"Custom filename for the chart image"`
}

// LineChartParams for CreateLineChart.
type LineChartParams struct {
	Data     interface{} `json:"data" description:"Dictionary with x-values as keys and numeric y-values, or list of dictionaries, or a JSON string of either." required:"true"`
	Title    string      `json:"title,omitempty" description:"Title of the chart (default \"Line Chart\")"`
	XLabel   string      `json:"x_label,omitempty" description:"Label for x-axis (default \"X-axis\")"`
	YLabel   string      `json:"y_label,omitempty" description:"Label for y-axis (default \"Y-axis\")"`
	Filename string      `json:"filename,omitempty" description:"Custom filename for the chart image"`
}

// PieChartParams for CreatePieChart.
type PieChartParams struct {
	Data     interface{} `json:"data" description:"Dictionary with categories as keys and numeric values, or list of dictionaries (label, value), or a JSON string of either." required:"true"`
	Title    string      `json:"title,omitempty" description:"Title of the chart (default \"Pie Chart\")"`
	Filename string      `json:"filename,omitempty" description:"Custom filename for the chart image"`
}

// ScatterParams for CreateScatterPlot.
type ScatterParams struct {
	XData    []float64   `json:"x_data,omitempty" description:"List of x-values (can also use 'x' parameter)"`
	YData    []float64   `json:"y_data,omitempty" description:"List of y-values (can also use 'y' parameter)"`
	Title    string      `json:"title,omitempty" description:"Title of the chart (default \"Scatter Plot\")"`
	XLabel   string      `json:"x_label,omitempty" description:"Label for x-axis"`
	YLabel   string      `json:"y_label,omitempty" description:"Label for y-axis"`
	Filename string      `json:"filename,omitempty" description:"Custom filename for the chart image"`
	X        []float64   `json:"x,omitempty" description:"Alias of x_data"`
	Y        []float64   `json:"y,omitempty" description:"Alias of y_data"`
	Data     interface{} `json:"data,omitempty" description:"Alternative format - list of [x,y] pairs or dict with 'x' and 'y' keys"`
}

// HistogramParams for CreateHistogram.
type HistogramParams struct {
	Data     []interface{} `json:"data" description:"List of numeric values to plot" required:"true"`
	Bins     int           `json:"bins,omitempty" description:"Number of bins for the histogram (default 10)"`
	Title    string        `json:"title,omitempty" description:"Title of the chart (default \"Histogram\")"`
	XLabel   string        `json:"x_label,omitempty" description:"Label for x-axis (default \"Values\")"`
	YLabel   string        `json:"y_label,omitempty" description:"Label for y-axis (default \"Frequency\")"`
	Filename string        `json:"filename,omitempty" description:"Custom filename for the chart image"`
}

// NewVisualizationTools creates the toolkit and the output directory.
func NewVisualizationTools(opts ...Option) (*VisualizationTools, error) {
	v := &VisualizationTools{outputDir: "charts", enableBar: true, enableLine: true, enablePie: true, enableScatter: true, enableHistogram: true}
	v.Toolkit = toolkit.NewToolkit()
	v.Toolkit.Name = "visualization_tools"
	v.Toolkit.Description = "Create bar, line, pie, scatter and histogram charts saved as SVG files."
	for _, o := range opts {
		o(v)
	}
	if err := os.MkdirAll(v.outputDir, 0o755); err != nil {
		return nil, err
	}
	if v.enableBar {
		v.Toolkit.Register("CreateBarChart", "Create a bar chart from the provided data. Returns JSON with chart information and file path.", v, v.CreateBarChart, BarChartParams{})
	}
	if v.enableLine {
		v.Toolkit.Register("CreateLineChart", "Create a line chart from the provided data. Returns JSON with chart information and file path.", v, v.CreateLineChart, LineChartParams{})
	}
	if v.enablePie {
		v.Toolkit.Register("CreatePieChart", "Create a pie chart from the provided data. Returns JSON with chart information and file path.", v, v.CreatePieChart, PieChartParams{})
	}
	if v.enableScatter {
		v.Toolkit.Register("CreateScatterPlot", "Create a scatter plot from the provided data. Returns JSON with chart information and file path.", v, v.CreateScatterPlot, ScatterParams{})
	}
	if v.enableHistogram {
		v.Toolkit.Register("CreateHistogram", "Create a histogram from the provided data. Returns JSON with chart information and file path.", v, v.CreateHistogram, HistogramParams{})
	}
	return v, nil
}

type point struct {
	label string
	value float64
}

func toFloat(v interface{}) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	}
	return 0, false
}

// normalize mirrors _normalize_data_for_charts.
func normalize(data interface{}) []point {
	if s, ok := data.(string); ok {
		var parsed interface{}
		if err := json.Unmarshal([]byte(s), &parsed); err == nil {
			data = parsed
		}
	}
	switch d := data.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(d))
		for k := range d {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make([]point, 0, len(keys))
		for _, k := range keys {
			f, _ := toFloat(d[k])
			out = append(out, point{k, f})
		}
		return out
	case []interface{}:
		if len(d) == 0 {
			break
		}
		if _, isMap := d[0].(map[string]interface{}); isMap {
			var out []point
			for _, item := range d {
				m, ok := item.(map[string]interface{})
				if !ok || len(m) < 2 {
					continue
				}
				keys := make([]string, 0, len(m))
				for k := range m {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				labelKey, valueKey := "", ""
				for _, k := range keys {
					if _, num := toFloat(m[k]); num {
						if valueKey == "" {
							valueKey = k
						}
					} else if labelKey == "" {
						labelKey = k
					}
				}
				if labelKey == "" {
					labelKey = keys[0]
				}
				f, _ := toFloat(m[valueKey])
				out = append(out, point{fmt.Sprint(m[labelKey]), f})
			}
			return out
		}
		out := make([]point, len(d))
		for i, v := range d {
			f, _ := toFloat(v)
			out[i] = point{fmt.Sprintf("Item %d", i+1), f}
		}
		return out
	}
	return []point{{"Data", 1}}
}

func (v *VisualizationTools) path(filename, prefix string) string {
	if filename == "" {
		entries, _ := os.ReadDir(v.outputDir)
		filename = fmt.Sprintf("%s_%d.svg", prefix, len(entries)+1)
	}
	filename = filepath.Base(filename)
	filename = strings.TrimSuffix(filename, filepath.Ext(filename)) + ".svg"
	return filepath.Join(v.outputDir, filename)
}

func result(chartType, title, path string, n int, extra map[string]interface{}, err error) (interface{}, error) {
	if err != nil {
		b, _ := json.Marshal(map[string]string{"chart_type": chartType, "error": err.Error(), "status": "error"})
		return string(b), nil
	}
	m := map[string]interface{}{"chart_type": chartType, "title": title, "file_path": path, "data_points": n, "status": "success"}
	for k, val := range extra {
		m[k] = val
	}
	b, _ := json.Marshal(m)
	return string(b), nil
}

func def(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

// CreateBarChart renders a bar chart.
func (v *VisualizationTools) CreateBarChart(p BarChartParams) (interface{}, error) {
	pts := normalize(p.Data)
	title := def(p.Title, "Bar Chart")
	path := v.path(p.Filename, "bar_chart")
	err := os.WriteFile(path, []byte(cartesianSVG("bar", title, def(p.XLabel, "Categories"), def(p.YLabel, "Values"), pts)), 0o644)
	return result("bar_chart", title, path, len(pts), nil, err)
}

// CreateLineChart renders a line chart.
func (v *VisualizationTools) CreateLineChart(p LineChartParams) (interface{}, error) {
	pts := normalize(p.Data)
	title := def(p.Title, "Line Chart")
	path := v.path(p.Filename, "line_chart")
	err := os.WriteFile(path, []byte(cartesianSVG("line", title, def(p.XLabel, "X-axis"), def(p.YLabel, "Y-axis"), pts)), 0o644)
	return result("line_chart", title, path, len(pts), nil, err)
}

// CreatePieChart renders a pie chart.
func (v *VisualizationTools) CreatePieChart(p PieChartParams) (interface{}, error) {
	pts := normalize(p.Data)
	title := def(p.Title, "Pie Chart")
	path := v.path(p.Filename, "pie_chart")
	err := os.WriteFile(path, []byte(pieSVG(title, pts)), 0o644)
	return result("pie_chart", title, path, len(pts), nil, err)
}

// CreateScatterPlot renders a scatter plot.
func (v *VisualizationTools) CreateScatterPlot(p ScatterParams) (interface{}, error) {
	xs, ys := p.XData, p.YData
	if xs == nil {
		xs = p.X
	}
	if ys == nil {
		ys = p.Y
	}
	data := p.Data
	if s, ok := data.(string); ok {
		var parsed interface{}
		if json.Unmarshal([]byte(s), &parsed) == nil {
			data = parsed
		}
	}
	switch d := data.(type) {
	case map[string]interface{}:
		if dx, ok := floats(d["x"]); ok {
			if dy, ok := floats(d["y"]); ok {
				xs, ys = dx, dy
			}
		}
	case []interface{}:
		if len(d) > 0 {
			if first, ok := d[0].([]interface{}); ok && len(first) == 2 {
				xs, ys = nil, nil
				for _, pt := range d {
					pair, _ := pt.([]interface{})
					if len(pair) == 2 {
						x, _ := toFloat(pair[0])
						y, _ := toFloat(pair[1])
						xs, ys = append(xs, x), append(ys, y)
					}
				}
			}
		}
	}
	title := def(p.Title, "Scatter Plot")
	if xs == nil || ys == nil {
		return result("scatter_plot", title, "", 0, nil, fmt.Errorf("Missing x_data and y_data parameters"))
	}
	if len(xs) != len(ys) {
		return result("scatter_plot", title, "", 0, nil, fmt.Errorf("x_data and y_data must have the same length"))
	}
	path := v.path(p.Filename, "scatter_plot")
	err := os.WriteFile(path, []byte(scatterSVG(title, def(p.XLabel, "X-axis"), def(p.YLabel, "Y-axis"), xs, ys)), 0o644)
	return result("scatter_plot", title, path, len(xs), nil, err)
}

func floats(v interface{}) ([]float64, bool) {
	list, ok := v.([]interface{})
	if !ok {
		return nil, false
	}
	out := make([]float64, 0, len(list))
	for _, x := range list {
		f, ok := toFloat(x)
		if !ok {
			return nil, false
		}
		out = append(out, f)
	}
	return out, true
}

// CreateHistogram renders a histogram.
func (v *VisualizationTools) CreateHistogram(p HistogramParams) (interface{}, error) {
	title := def(p.Title, "Histogram")
	bins := p.Bins
	if bins <= 0 {
		bins = 10
	}
	if len(p.Data) == 0 {
		return result("histogram", title, "", 0, nil, fmt.Errorf("Data must be a non-empty list of numbers"))
	}
	var nums []float64
	for _, d := range p.Data {
		if f, ok := toFloat(d); ok {
			nums = append(nums, f)
		} else if s, ok := d.(string); ok {
			if f, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
				nums = append(nums, f)
			}
		}
	}
	if len(nums) == 0 {
		return result("histogram", title, "", 0, nil, fmt.Errorf("No valid numeric data found"))
	}
	lo, hi := nums[0], nums[0]
	for _, n := range nums {
		lo, hi = math.Min(lo, n), math.Max(hi, n)
	}
	if hi == lo {
		lo, hi = lo-0.5, hi+0.5
	}
	width := (hi - lo) / float64(bins)
	counts := make([]float64, bins)
	for _, n := range nums {
		i := int((n - lo) / width)
		if i >= bins {
			i = bins - 1
		}
		counts[i]++
	}
	pts := make([]point, bins)
	for i := range counts {
		pts[i] = point{fmt.Sprintf("%.3g", lo+width*float64(i)), counts[i]}
	}
	path := v.path(p.Filename, "histogram")
	err := os.WriteFile(path, []byte(cartesianSVG("hist", title, def(p.XLabel, "Values"), def(p.YLabel, "Frequency"), pts)), 0o644)
	return result("histogram", title, path, len(nums), map[string]interface{}{"bins": bins}, err)
}

// ---- SVG rendering ----

const (
	svgW, svgH                     = 1000.0, 600.0
	padL, padR, padT, padB         = 80.0, 30.0, 60.0, 110.0
	plotW, plotH                   = svgW - padL - padR, svgH - padT - padB
	barColor, lineColor, gridColor = "#4C72B0", "#DD8452", "#e5e5e5"
)

var palette = []string{"#4C72B0", "#DD8452", "#55A868", "#C44E52", "#8172B3", "#937860", "#DA8BC3", "#8C8C8C", "#CCB974", "#64B5CD"}

func esc(s string) string { return html.EscapeString(s) }

func header(title string) *strings.Builder {
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f" font-family="sans-serif">`, svgW, svgH, svgW, svgH)
	fmt.Fprintf(&b, `<rect width="100%%" height="100%%" fill="white"/><text x="%.0f" y="35" font-size="22" text-anchor="middle">%s</text>`, svgW/2, esc(title))
	return &b
}

func axes(b *strings.Builder, xLabel, yLabel string, lo, hi float64) func(float64) float64 {
	if hi == lo {
		hi = lo + 1
	}
	y := func(v float64) float64 { return padT + plotH - (v-lo)/(hi-lo)*plotH }
	for i := 0; i <= 5; i++ {
		val := lo + (hi-lo)*float64(i)/5
		yy := y(val)
		fmt.Fprintf(b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s"/>`, padL, yy, padL+plotW, yy, gridColor)
		fmt.Fprintf(b, `<text x="%.1f" y="%.1f" font-size="12" text-anchor="end">%s</text>`, padL-8, yy+4, strconv.FormatFloat(val, 'g', 4, 64))
	}
	fmt.Fprintf(b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="black"/>`, padL, padT+plotH, padL+plotW, padT+plotH)
	fmt.Fprintf(b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="black"/>`, padL, padT, padL, padT+plotH)
	fmt.Fprintf(b, `<text x="%.1f" y="%.1f" font-size="15" text-anchor="middle">%s</text>`, padL+plotW/2, svgH-15, esc(xLabel))
	fmt.Fprintf(b, `<text x="20" y="%.1f" font-size="15" text-anchor="middle" transform="rotate(-90 20 %.1f)">%s</text>`, padT+plotH/2, padT+plotH/2, esc(yLabel))
	return y
}

func bounds(vals []float64, zero bool) (float64, float64) {
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, v := range vals {
		lo, hi = math.Min(lo, v), math.Max(hi, v)
	}
	if len(vals) == 0 {
		return 0, 1
	}
	if zero {
		lo, hi = math.Min(lo, 0), math.Max(hi, 0)
	}
	return lo, hi
}

func cartesianSVG(kind, title, xLabel, yLabel string, pts []point) string {
	b := header(title)
	vals := make([]float64, len(pts))
	for i, p := range pts {
		vals[i] = p.value
	}
	lo, hi := bounds(vals, kind != "line")
	y := axes(b, xLabel, yLabel, lo, hi)
	n := float64(len(pts))
	step := plotW / math.Max(n, 1)
	var line []string
	for i, p := range pts {
		cx := padL + step*(float64(i)+0.5)
		switch kind {
		case "bar", "hist":
			w := step * 0.8
			if kind == "hist" {
				w = step
			}
			top, base := y(math.Max(p.value, 0)), y(math.Min(p.value, 0))
			fmt.Fprintf(b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="%s" stroke="white"/>`, cx-w/2, top, w, base-top, barColor)
		case "line":
			line = append(line, fmt.Sprintf("%.1f,%.1f", cx, y(p.value)))
			fmt.Fprintf(b, `<circle cx="%.1f" cy="%.1f" r="4" fill="%s"/>`, cx, y(p.value), lineColor)
		}
		fmt.Fprintf(b, `<text x="%.1f" y="%.1f" font-size="12" text-anchor="end" transform="rotate(-45 %.1f %.1f)">%s</text>`, cx, padT+plotH+18, cx, padT+plotH+18, esc(p.label))
	}
	if len(line) > 0 {
		fmt.Fprintf(b, `<polyline points="%s" fill="none" stroke="%s" stroke-width="2"/>`, strings.Join(line, " "), lineColor)
	}
	b.WriteString("</svg>")
	return b.String()
}

func scatterSVG(title, xLabel, yLabel string, xs, ys []float64) string {
	b := header(title)
	ylo, yhi := bounds(ys, false)
	xlo, xhi := bounds(xs, false)
	if xhi == xlo {
		xhi = xlo + 1
	}
	y := axes(b, xLabel, yLabel, ylo, yhi)
	for i := 0; i <= 5; i++ {
		val := xlo + (xhi-xlo)*float64(i)/5
		xx := padL + (val-xlo)/(xhi-xlo)*plotW
		fmt.Fprintf(b, `<text x="%.1f" y="%.1f" font-size="12" text-anchor="middle">%s</text>`, xx, padT+plotH+18, strconv.FormatFloat(val, 'g', 4, 64))
	}
	for i := range xs {
		fmt.Fprintf(b, `<circle cx="%.1f" cy="%.1f" r="5" fill="%s" fill-opacity="0.7"/>`, padL+(xs[i]-xlo)/(xhi-xlo)*plotW, y(ys[i]), barColor)
	}
	b.WriteString("</svg>")
	return b.String()
}

func pieSVG(title string, pts []point) string {
	b := header(title)
	total := 0.0
	for _, p := range pts {
		total += math.Max(p.value, 0)
	}
	cx, cy, r := svgW/2-120, svgH/2+20, 220.0
	angle := -math.Pi / 2
	for i, p := range pts {
		v := math.Max(p.value, 0)
		if total == 0 || v == 0 {
			continue
		}
		frac := v / total
		end := angle + frac*2*math.Pi
		color := palette[i%len(palette)]
		if frac >= 0.9999 {
			fmt.Fprintf(b, `<circle cx="%.1f" cy="%.1f" r="%.1f" fill="%s"/>`, cx, cy, r, color)
		} else {
			large := 0
			if frac > 0.5 {
				large = 1
			}
			fmt.Fprintf(b, `<path d="M%.1f,%.1f L%.1f,%.1f A%.1f,%.1f 0 %d 1 %.1f,%.1f Z" fill="%s" stroke="white"/>`,
				cx, cy, cx+r*math.Cos(angle), cy+r*math.Sin(angle), r, r, large, cx+r*math.Cos(end), cy+r*math.Sin(end), color)
		}
		mid := (angle + end) / 2
		fmt.Fprintf(b, `<text x="%.1f" y="%.1f" font-size="13" text-anchor="middle" fill="white">%.1f%%</text>`, cx+r*0.6*math.Cos(mid), cy+r*0.6*math.Sin(mid), frac*100)
		ly := 90 + float64(i)*24
		fmt.Fprintf(b, `<rect x="%.1f" y="%.1f" width="14" height="14" fill="%s"/><text x="%.1f" y="%.1f" font-size="14">%s</text>`, svgW-250, ly, color, svgW-230, ly+12, esc(p.label))
		angle = end
	}
	b.WriteString("</svg>")
	return b.String()
}
