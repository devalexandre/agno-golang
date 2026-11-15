package functions

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"
)

// The helpers below decode JSON keeping object key order and render values the
// way Python's json/str do, so the functions produce the same text as upstream
// Agno (column order in json_to_csv, pretty-printing in extract_json, ...).

// jsonObject is a JSON object that remembers key insertion order. A duplicated
// key keeps its first position and its last value, like a Python dict.
type jsonObject struct {
	keys []string
	vals map[string]interface{}
}

func (o *jsonObject) set(key string, val interface{}) {
	if _, ok := o.vals[key]; !ok {
		o.keys = append(o.keys, key)
	}
	o.vals[key] = val
}

// noJSON marks the absence of a decodable payload.
type noJSON struct{}

// rawDecode decodes exactly one JSON value at the start of text and returns it
// with the number of bytes it spans. Trailing text is ignored.
func rawDecode(text string) (interface{}, int, error) {
	dec := json.NewDecoder(strings.NewReader(text))
	dec.UseNumber()
	val, err := decodeValue(dec)
	if err != nil {
		return nil, 0, err
	}
	return val, int(dec.InputOffset()), nil
}

func decodeValue(dec *json.Decoder) (interface{}, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	return decodeFrom(dec, tok)
}

func decodeFrom(dec *json.Decoder, tok json.Token) (interface{}, error) {
	delim, ok := tok.(json.Delim)
	if !ok {
		return tok, nil
	}
	switch delim {
	case '{':
		obj := &jsonObject{vals: map[string]interface{}{}}
		for {
			t, err := dec.Token()
			if err != nil {
				return nil, err
			}
			if d, ok := t.(json.Delim); ok && d == '}' {
				return obj, nil
			}
			key, ok := t.(string)
			if !ok {
				return nil, io.ErrUnexpectedEOF
			}
			val, err := decodeValue(dec)
			if err != nil {
				return nil, err
			}
			obj.set(key, val)
		}
	case '[':
		arr := []interface{}{}
		for {
			t, err := dec.Token()
			if err != nil {
				return nil, err
			}
			if d, ok := t.(json.Delim); ok && d == ']' {
				return arr, nil
			}
			val, err := decodeFrom(dec, t)
			if err != nil {
				return nil, err
			}
			arr = append(arr, val)
		}
	}
	return nil, io.ErrUnexpectedEOF
}

// jsonPayload decodes the largest JSON object or array in text, or returns
// noJSON{} when none decodes. Largest, not first: a gathering agent's "[1]"
// citations and "[ ]" checkboxes decode too.
func jsonPayload(text string) interface{} {
	var best interface{} = noJSON{}
	bestSpan, skipUntil := 0, 0
	for start := 0; start < len(text); start++ {
		c := text[start]
		if c != '[' && c != '{' {
			continue
		}
		if start < skipUntil { // inside a value already decoded
			continue
		}
		val, n, err := rawDecode(text[start:])
		if err != nil {
			continue
		}
		end := start + n
		// Spans are compared in characters, as Python does.
		if span := utf8.RuneCountInString(text[start:end]); span > bestSpan {
			bestSpan, best = span, val
		}
		skipUntil = end
	}
	return best
}

// dumpJSON renders v like Python's json.dumps(v, ensure_ascii=False), with
// indent=2 when pretty is true and the default ", " / ": " separators otherwise.
func dumpJSON(v interface{}, pretty bool) string {
	var b strings.Builder
	writeJSON(&b, v, pretty, 0)
	return b.String()
}

func writeJSON(b *strings.Builder, v interface{}, pretty bool, depth int) {
	newline := func(d int) {
		if pretty {
			b.WriteByte('\n')
			b.WriteString(strings.Repeat("  ", d))
		}
	}
	itemSep := ", "
	if pretty {
		itemSep = ","
	}
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		if x {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case json.Number:
		b.WriteString(pyNumber(x, true))
	case string:
		writeJSONString(b, x)
	case *jsonObject:
		if len(x.keys) == 0 {
			b.WriteString("{}")
			return
		}
		b.WriteByte('{')
		for i, k := range x.keys {
			if i > 0 {
				b.WriteString(itemSep)
			}
			newline(depth + 1)
			writeJSONString(b, k)
			b.WriteString(": ")
			writeJSON(b, x.vals[k], pretty, depth+1)
		}
		newline(depth)
		b.WriteByte('}')
	case []interface{}:
		if len(x) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteByte('[')
		for i, item := range x {
			if i > 0 {
				b.WriteString(itemSep)
			}
			newline(depth + 1)
			writeJSON(b, item, pretty, depth+1)
		}
		newline(depth)
		b.WriteByte(']')
	default:
		data, err := marshalNoEscape(x, "")
		if err != nil {
			b.WriteString(strconv.Quote(err.Error()))
			return
		}
		b.WriteString(data)
	}
}

func writeJSONString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			if r < 0x20 {
				b.WriteString(`\u00`)
				b.WriteString(strconv.FormatInt(int64(r)>>4, 16))
				b.WriteString(strconv.FormatInt(int64(r)&0xf, 16))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}

// pyNumber renders a JSON number the way Python prints the int or float it
// decodes to. inJSON selects json.dumps spelling for infinities.
func pyNumber(n json.Number, inJSON bool) string {
	s := string(n)
	if !strings.ContainsAny(s, ".eE") {
		if i, ok := new(big.Int).SetString(s, 10); ok {
			return i.String()
		}
		return s
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil && !math.IsInf(f, 0) {
		return s
	}
	return pyFloat(f, inJSON)
}

// pyFloat mirrors Python's float repr.
func pyFloat(f float64, inJSON bool) string {
	switch {
	case math.IsInf(f, 1):
		if inJSON {
			return "Infinity"
		}
		return "inf"
	case math.IsInf(f, -1):
		if inJSON {
			return "-Infinity"
		}
		return "-inf"
	case math.IsNaN(f):
		if inJSON {
			return "NaN"
		}
		return "nan"
	}
	e := strconv.FormatFloat(f, 'e', -1, 64)
	exp, _ := strconv.Atoi(e[strings.IndexByte(e, 'e')+1:])
	if exp >= -4 && exp < 16 {
		s := strconv.FormatFloat(f, 'f', -1, 64)
		if !strings.Contains(s, ".") {
			s += ".0"
		}
		return s
	}
	return e
}

// pyStr renders a decoded JSON value like Python's str(), with nested objects
// and arrays JSON-encoded (the json_to_csv cell rule). nil renders as "".
func pyStr(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		if x {
			return "True"
		}
		return "False"
	case json.Number:
		return pyNumber(x, false)
	case *jsonObject, []interface{}:
		return dumpJSON(x, false)
	}
	return dumpJSON(v, false)
}

// marshalNoEscape is json.MarshalIndent without HTML escaping.
func marshalNoEscape(v interface{}, indent string) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if indent != "" {
		enc.SetIndent("", indent)
	}
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimSuffix(buf.String(), "\n"), nil
}
