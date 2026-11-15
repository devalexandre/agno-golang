package chart

import (
	"strings"
	"testing"
)

func TestCreateChart(t *testing.T) {
	c := NewChartTools("")
	out, err := c.Execute("chart_tools_CreateChart", []byte(`{
		"chart_type":"line",
		"data":[{"month":"Jan","revenue":"4000","cost":10},{"month":"Feb","revenue":4500.5,"cost":12}],
		"config":{"revenue":"Revenue","cost":"Cost"},
		"x_key":"month","title":"Rev"}`))
	if err != nil {
		t.Fatal(err)
	}
	s := out.(string)
	if !strings.HasPrefix(s, "```chart\n") || !strings.Contains(s, `"revenue": 4000`) ||
		!strings.Contains(s, `"yKeys": [`) || !strings.Contains(s, `"label": "Revenue"`) {
		t.Fatal(s)
	}
	var ol OrderedLabels
	if err := ol.UnmarshalJSON([]byte(`{"revenue":"R","cost":"C"}`)); err != nil || ol.Keys[0] != "revenue" || ol.Keys[1] != "cost" {
		t.Fatal(ol, err)
	}
	out, _ = c.CreateChart(CreateChartParams{ChartType: "pie", Data: []map[string]interface{}{{"p": "a", "n": 1.0}},
		Config: NewOrderedLabels("n", "Runs"), NameKey: "p"})
	if !strings.Contains(out.(string), "pie charts require both name_key and value_key") {
		t.Fatal(out)
	}
	out, _ = c.CreateChart(CreateChartParams{ChartType: "bar", Data: []map[string]interface{}{{"m": "a", "v": "x"}},
		Config: NewOrderedLabels("v", "V"), XKey: "m"})
	if !strings.HasPrefix(out.(string), "Chart validation failed: y_keys 'v' must be numeric") {
		t.Fatal(out)
	}
	out, _ = c.CreateChart(CreateChartParams{ChartType: "radar"})
	if !strings.Contains(out.(string), "unknown chart_type 'radar'") {
		t.Fatal(out)
	}
}
