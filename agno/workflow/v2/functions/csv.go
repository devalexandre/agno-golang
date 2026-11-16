package functions

import (
	"errors"
	"strings"
)

// The CSV helpers implement Python's csv module "excel" dialect (comma
// delimiter, double-quote quoting, QUOTE_MINIMAL, CRLF line terminator, strict
// parsing) so that json_to_csv / csv_to_markdown_table match upstream output.
// encoding/csv differs in small ways (it quotes fields with a leading space and
// rejects bare quotes inside unquoted fields), hence the dedicated code.

// csvFieldLimit is Python's default csv.field_size_limit().
const csvFieldLimit = 131072

var errCSV = errors.New("invalid CSV")

// writeCSVRow appends one row with QUOTE_MINIMAL quoting and a CRLF terminator.
func writeCSVRow(b *strings.Builder, fields []string) {
	if len(fields) == 1 && fields[0] == "" {
		// Python writes a lone empty field quoted so the row is not blank.
		b.WriteString(`""` + "\r\n")
		return
	}
	for i, f := range fields {
		if i > 0 {
			b.WriteByte(',')
		}
		if strings.ContainsAny(f, ",\"\r\n") {
			b.WriteByte('"')
			b.WriteString(strings.ReplaceAll(f, `"`, `""`))
			b.WriteByte('"')
		} else {
			b.WriteString(f)
		}
	}
	b.WriteString("\r\n")
}

type csvState int

const (
	csvStartRecord csvState = iota
	csvStartField
	csvInField
	csvInQuoted
	csvQuoteInQuoted
)

// readCSV parses text into rows, dropping blank lines like
// `row for row in csv.reader(...) if row`. It fails on a character after a
// closing quote, an unterminated quoted field, or a field over the size limit.
func readCSV(text string) ([][]string, error) {
	var (
		rows     [][]string
		row      []string
		field    []rune
		state    = csvStartRecord
		runes    = []rune(text)
		addField = func() {
			row = append(row, string(field))
			field = field[:0]
		}
		addChar = func(r rune) error {
			if len(field) >= csvFieldLimit {
				return errCSV
			}
			field = append(field, r)
			return nil
		}
		endRecord = func() {
			rows = append(rows, row)
			row = nil
		}
	)

	for i := 0; i < len(runes); i++ {
		c := runes[i]
		isEOL := c == '\n' || c == '\r'
		if isEOL && c == '\r' && i+1 < len(runes) && runes[i+1] == '\n' && state != csvInQuoted {
			i++ // treat CRLF as a single line end
		}
		switch state {
		case csvStartRecord:
			if isEOL {
				continue // blank line
			}
			state = csvStartField
			fallthrough
		case csvStartField:
			switch {
			case c == '"':
				state = csvInQuoted
			case c == ',':
				addField()
			case isEOL:
				addField()
				endRecord()
				state = csvStartRecord
			default:
				if err := addChar(c); err != nil {
					return nil, err
				}
				state = csvInField
			}
		case csvInField:
			switch {
			case c == ',':
				addField()
				state = csvStartField
			case isEOL:
				addField()
				endRecord()
				state = csvStartRecord
			default:
				if err := addChar(c); err != nil {
					return nil, err
				}
			}
		case csvInQuoted:
			if c == '"' {
				state = csvQuoteInQuoted
			} else if err := addChar(c); err != nil {
				return nil, err
			}
		case csvQuoteInQuoted:
			switch {
			case c == '"':
				if err := addChar('"'); err != nil {
					return nil, err
				}
				state = csvInQuoted
			case c == ',':
				addField()
				state = csvStartField
			case isEOL:
				addField()
				endRecord()
				state = csvStartRecord
			default:
				return nil, errCSV // strict: '"' followed by a stray character
			}
		}
	}

	switch state {
	case csvInQuoted:
		return nil, errCSV // unexpected end of data
	case csvStartField, csvInField, csvQuoteInQuoted:
		addField()
		endRecord()
	}
	return rows, nil
}
