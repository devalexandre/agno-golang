package visualization

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func decode(t *testing.T, out interface{}) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(out.(string)), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestVisualization(t *testing.T) {
	v, err := NewVisualizationTools(WithOutputDir(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	out, _ := v.Execute("visualization_tools_CreateBarChart", []byte(`{"data":{"A":3,"B":5},"title":"<T>"}`))
	m := decode(t, out)
	if m["status"] != "success" || m["data_points"] != 2.0 || !strings.HasSuffix(m["file_path"].(string), "bar_chart_1.svg") {
		t.Fatal(m)
	}
	svg, _ := os.ReadFile(m["file_path"].(string))
	if !strings.HasPrefix(string(svg), "<svg") || !strings.Contains(string(svg), "&lt;T&gt;") {
		t.Fatal(string(svg))
	}
	m = decode(t, mustOut(v.CreateLineChart(LineChartParams{Data: `[{"month":"Jan","v":1},{"month":"Feb","v":2}]`, Filename: "l.png"})))
	if !strings.HasSuffix(m["file_path"].(string), "l.svg") || m["data_points"] != 2.0 {
		t.Fatal(m)
	}
	m = decode(t, mustOut(v.CreatePieChart(PieChartParams{Data: []interface{}{1.0, 2.0, 3.0}})))
	if m["data_points"] != 3.0 {
		t.Fatal(m)
	}
	m = decode(t, mustOut(v.CreateScatterPlot(ScatterParams{Data: []interface{}{[]interface{}{1.0, 2.0}, []interface{}{3.0, 4.0}}})))
	if m["data_points"] != 2.0 {
		t.Fatal(m)
	}
	m = decode(t, mustOut(v.CreateScatterPlot(ScatterParams{X: []float64{1}, Y: []float64{1, 2}})))
	if m["status"] != "error" {
		t.Fatal(m)
	}
	m = decode(t, mustOut(v.CreateHistogram(HistogramParams{Data: []interface{}{1.0, "2", "x", 3.0}, Bins: 2})))
	if m["data_points"] != 3.0 || m["bins"] != 2.0 {
		t.Fatal(m)
	}
}

func mustOut(out interface{}, _ error) interface{} { return out }
