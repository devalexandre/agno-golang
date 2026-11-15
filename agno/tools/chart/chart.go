// Package chart ports Agno's ChartTools: it validates chart data and returns
// a fenced ```chart block (JSON ChartSpec) that Agno OS renders inline.
package chart

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// Supported chart types.
var ChartTypes = []string{"bar", "line", "area", "pie", "bar-horizontal", "bar-stacked", "area-stacked"}

// DefaultInstructions mirrors ChartTools.DEFAULT_INSTRUCTIONS.
const DefaultInstructions = `You can render interactive charts inline in your responses.

When the user asks for trends, comparisons, usage, costs, performance, or
any answer that benefits from a visualization, call the ` + "`create_chart`" + `
tool with the chart data. The tool validates the data and returns a
fenced ` + "`chart`" + ` code block.

Rules:
- Include the returned ` + "`chart`" + ` block verbatim in your markdown response.
  Do not modify, reformat, or wrap it.
- Add one short sentence before the chart describing what it shows, and
  one short sentence after it with the key takeaway.
- Do not return tables that duplicate the chart data unless asked.
- Keep the data compact enough to inspect in chat. Aggregate or sample
  if the raw dataset is large.
- If the tool returns a validation error, fix the data and call it again.
`

// OrderedLabels is a JSON object of series key -> label that keeps key order.
// Note: toolkit.Execute normalises arguments through a Go map, so when called
// by an agent the default yKeys order is alphabetical; pass y_keys for an
// explicit order.
type OrderedLabels struct {
	Keys   []string
	Labels map[string]string
}

// UnmarshalJSON decodes an object preserving key order.
func (o *OrderedLabels) UnmarshalJSON(b []byte) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return fmt.Errorf("config must be an object")
	}
	o.Labels = map[string]string{}
	o.Keys = nil
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return err
		}
		key := kt.(string)
		var v interface{}
		if err := dec.Decode(&v); err != nil {
			return err
		}
		if _, dup := o.Labels[key]; !dup {
			o.Keys = append(o.Keys, key)
		}
		o.Labels[key] = fmt.Sprint(v)
	}
	return nil
}

// NewOrderedLabels builds OrderedLabels from key/label pairs.
func NewOrderedLabels(pairs ...string) OrderedLabels {
	o := OrderedLabels{Labels: map[string]string{}}
	for i := 0; i+1 < len(pairs); i += 2 {
		o.Keys = append(o.Keys, pairs[i])
		o.Labels[pairs[i]] = pairs[i+1]
	}
	return o
}

// CreateChartParams are CreateChart's arguments.
type CreateChartParams struct {
	ChartType   string                   `json:"chart_type" description:"One of \"bar\", \"line\", \"area\", \"pie\", \"bar-horizontal\", \"bar-stacked\", \"area-stacked\"." required:"true"`
	Data        []map[string]interface{} `json:"data" description:"Flat rows, for example [{\"month\": \"Jan\", \"revenue\": 4000}]. Use strings for labels/dates and numbers for measured values." required:"true"`
	Config      OrderedLabels            `json:"config" description:"Maps each series key to its display label, for example {\"revenue\": \"Revenue ($)\"}." required:"true"`
	Title       *string                  `json:"title,omitempty" description:"Optional heading shown above the chart."`
	Description *string                  `json:"description,omitempty" description:"Optional supporting copy shown with the chart."`
	Value       *string                  `json:"value,omitempty" description:"Optional preformatted headline figure (e.g. \"200M\", \"$22.3K\", \"94%\"). Leave it out when no single number summarizes the chart; never invent one."`
	XKey        string                   `json:"x_key,omitempty" description:"Category or date field for cartesian charts, for example \"month\". Required for non-pie charts."`
	YKeys       []string                 `json:"y_keys,omitempty" description:"Explicit series order for cartesian charts. Defaults to the config keys."`
	NameKey     string                   `json:"name_key,omitempty" description:"Slice label field for pie charts, for example \"provider\". Required for pie charts."`
	ValueKey    string                   `json:"value_key,omitempty" description:"Numeric slice value field for pie charts, for example \"runs\". Required for pie charts."`
}

// ChartTools builds validated chart blocks.
type ChartTools struct {
	toolkit.Toolkit
	// Instructions to add to the agent's system prompt.
	Instructions string
}

// NewChartTools creates the toolkit. An empty instructions uses DefaultInstructions.
func NewChartTools(instructions string) *ChartTools {
	if instructions == "" {
		instructions = DefaultInstructions
	}
	c := &ChartTools{Instructions: instructions}
	c.Toolkit = toolkit.NewToolkit()
	c.Toolkit.Name = "chart_tools"
	c.Toolkit.Description = "Build validated chart blocks that Agno OS renders inline in chat."
	c.Toolkit.Register("CreateChart", "Build a chart block that Agno OS renders inline in the chat. Include the returned fenced block verbatim in your markdown response. Returns a fenced ```chart block, or an error message starting with \"Chart validation failed\".", c, c.CreateChart, CreateChartParams{})
	return c
}

// GetInstructions returns the toolkit instructions.
func (c *ChartTools) GetInstructions() string { return c.Instructions }

// CreateChart validates the arguments and returns a fenced chart block.
func (c *ChartTools) CreateChart(p CreateChartParams) (interface{}, error) {
	spec, err := BuildSpec(p)
	if err != nil {
		return fmt.Sprintf("Chart validation failed: %v. Fix the arguments and call create_chart again.", err), nil
	}
	b, _ := json.MarshalIndent(spec, "", "  ")
	return "```chart\n" + string(b) + "\n```", nil
}

// Spec is the chart payload rendered by Agno OS.
type Spec struct {
	Type        string                       `json:"type"`
	Data        []map[string]interface{}     `json:"data"`
	Config      map[string]map[string]string `json:"config"`
	Title       *string                      `json:"title,omitempty"`
	Description *string                      `json:"description,omitempty"`
	Value       *string                      `json:"value,omitempty"`
	XKey        string                       `json:"xKey,omitempty"`
	YKeys       []string                     `json:"yKeys,omitempty"`
	NameKey     string                       `json:"nameKey,omitempty"`
	ValueKey    string                       `json:"valueKey,omitempty"`
}

// BuildSpec validates arguments and assembles the spec.
func BuildSpec(p CreateChartParams) (*Spec, error) {
	valid := false
	for _, t := range ChartTypes {
		if t == p.ChartType {
			valid = true
		}
	}
	if !valid {
		return nil, fmt.Errorf("unknown chart_type '%s'. Use one of: %s", p.ChartType, strings.Join(ChartTypes, ", "))
	}
	if len(p.Data) == 0 {
		return nil, fmt.Errorf("data must be a non-empty list of rows")
	}
	if len(p.Config.Keys) == 0 {
		return nil, fmt.Errorf("config must map each series key to a label")
	}
	rowKeys := map[string]bool{}
	for i, row := range p.Data {
		for k, v := range row {
			switch v.(type) {
			case string, float64, float32, int, int64, json.Number:
			default:
				return nil, fmt.Errorf("data[%d]['%s'] must be a string or number, got %s. Rows must be flat objects", i, k, typeName(v))
			}
			rowKeys[k] = true
		}
	}
	var missing []string
	for _, k := range p.Config.Keys {
		if !rowKeys[k] {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("config keys not present in data rows: %v", missing)
	}
	spec := &Spec{Type: p.ChartType, Data: p.Data, Config: map[string]map[string]string{},
		Title: p.Title, Description: p.Description, Value: p.Value}
	for _, k := range p.Config.Keys {
		spec.Config[k] = map[string]string{"label": p.Config.Labels[k]}
	}
	if p.ChartType == "pie" {
		if p.NameKey == "" || p.ValueKey == "" {
			return nil, fmt.Errorf("pie charts require both name_key and value_key")
		}
		if err := requireKey(p.Data, p.NameKey, "name_key"); err != nil {
			return nil, err
		}
		if err := requireNumeric(p.Data, p.ValueKey, "value_key"); err != nil {
			return nil, err
		}
		if _, ok := p.Config.Labels[p.ValueKey]; !ok {
			return nil, fmt.Errorf("config must include a label for value_key '%s'", p.ValueKey)
		}
		spec.NameKey, spec.ValueKey = p.NameKey, p.ValueKey
		return spec, nil
	}
	if p.XKey == "" {
		return nil, fmt.Errorf("%s charts require x_key for the category or date field", p.ChartType)
	}
	if err := requireKey(p.Data, p.XKey, "x_key"); err != nil {
		return nil, err
	}
	yKeys := p.YKeys
	if len(yKeys) == 0 {
		for _, k := range p.Config.Keys {
			if k != p.XKey {
				yKeys = append(yKeys, k)
			}
		}
	}
	if len(yKeys) == 0 {
		return nil, fmt.Errorf("no series keys found: provide y_keys or config keys other than x_key")
	}
	for _, k := range yKeys {
		if err := requireNumeric(p.Data, k, "y_keys"); err != nil {
			return nil, err
		}
	}
	spec.XKey, spec.YKeys = p.XKey, yKeys
	return spec, nil
}

func typeName(v interface{}) string {
	switch v.(type) {
	case nil:
		return "NoneType"
	case bool:
		return "bool"
	case []interface{}:
		return "list"
	case map[string]interface{}:
		return "dict"
	}
	return fmt.Sprintf("%T", v)
}

func requireKey(data []map[string]interface{}, key, arg string) error {
	var missing []int
	for i, row := range data {
		if _, ok := row[key]; !ok && len(missing) < 5 {
			missing = append(missing, i)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s '%s' is missing from data rows at indexes %v", arg, key, missing)
	}
	return nil
}

func coerceNumber(s string) interface{} {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return s
	}
	if f == math.Trunc(f) && !strings.Contains(s, ".") && !strings.Contains(strings.ToLower(s), "e") {
		return int64(f)
	}
	return f
}

func requireNumeric(data []map[string]interface{}, key, arg string) error {
	for i, row := range data {
		v, ok := row[key]
		if !ok {
			return fmt.Errorf("%s '%s' is missing from data[%d]", arg, key, i)
		}
		if s, isStr := v.(string); isStr {
			v = coerceNumber(s)
			row[key] = v
		}
		switch v.(type) {
		case float64, float32, int, int64, json.Number:
		default:
			return fmt.Errorf("%s '%s' must be numeric in every row; data[%d]['%s'] is '%v'", arg, key, i, key, row[key])
		}
	}
	return nil
}
