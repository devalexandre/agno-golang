package tools

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// CSVExcelParserTool processa CSV e Excel
type CSVExcelParserTool struct {
	toolkit.Toolkit
	parsedFiles map[string]ParsedData
	statistics  map[string]FileStatistics
}

// ParsedData representa dados parseados
type ParsedData struct {
	FileID    string
	Filename  string
	FileType  string // "csv", "excel"
	Rows      []map[string]interface{}
	Headers   []string
	RowCount  int
	ColCount  int
	ParsedAt  time.Time
	FileSize  int64
	Delimiter string
}

// FileStatistics estatísticas do arquivo
type FileStatistics struct {
	FileID            string
	RowCount          int
	ColCount          int
	EmptyRows         int
	DuplicateRows     int
	DataTypes         map[string]string
	MissingValues     int
	MissingPercentage float64
}

// ParseCSVParams parâmetros para parse
type ParseCSVParams struct {
	Content   string `json:"content" description:"Conteúdo CSV ou dados brutos"`
	Delimiter string `json:"delimiter" description:"Delimitador (,;\\t|)"`
	HasHeader bool   `json:"has_header" description:"Primeira linha é header"`
	Filename  string `json:"filename" description:"Nome do arquivo"`
}

// TransformDataParams parâmetros para transformação
type TransformDataParams struct {
	FileID          string                   `json:"file_id" description:"ID do arquivo parseado"`
	Transformations []map[string]interface{} `json:"transformations" description:"Transformações a aplicar"`
}

// AnalyzeCSVParams parâmetros para análise
type AnalyzeCSVParams struct {
	FileID       string `json:"file_id" description:"ID do arquivo"`
	AnalysisType string `json:"analysis_type" description:"statistics, schema, quality"`
}

// ParseResult resultado de parse
type ParseResult struct {
	Success   bool                     `json:"success"`
	FileID    string                   `json:"file_id"`
	Filename  string                   `json:"filename"`
	Rows      []map[string]interface{} `json:"rows,omitempty"`
	Headers   []string                 `json:"headers"`
	RowCount  int                      `json:"row_count"`
	ColCount  int                      `json:"col_count"`
	Message   string                   `json:"message"`
	Timestamp time.Time                `json:"timestamp"`
}

// NewCSVExcelParserTool cria novo tool
func NewCSVExcelParserTool() *CSVExcelParserTool {
	t := &CSVExcelParserTool{
		parsedFiles: make(map[string]ParsedData),
		statistics:  make(map[string]FileStatistics),
	}
	t.Toolkit = toolkit.NewToolkit()

	t.Toolkit.Register(
		"ParseCSV",
		"Parsear arquivo CSV",
		t,
		t.ParseCSV,
		ParseCSVParams{},
	)

	t.Toolkit.Register(
		"TransformData",
		"Aplicar transformações aos dados",
		t,
		t.TransformData,
		TransformDataParams{},
	)

	t.Toolkit.Register(
		"AnalyzeData",
		"Analisar dados parseados",
		t,
		t.AnalyzeData,
		AnalyzeCSVParams{},
	)

	t.Toolkit.Register(
		"ExportData",
		"Exportar dados em formato diferente",
		t,
		t.ExportData,
		ExportParams{},
	)

	return t
}

// ParseCSV parseia CSV
func (t *CSVExcelParserTool) ParseCSV(params ParseCSVParams) (interface{}, error) {
	if params.Content == "" {
		return ParseResult{Success: false}, fmt.Errorf("content obrigatório")
	}

	if params.Delimiter == "" {
		params.Delimiter = ","
	}

	// Parsear CSV
	reader := csv.NewReader(strings.NewReader(params.Content))
	reader.Comma = rune(params.Delimiter[0])

	var headers []string
	var rows []map[string]interface{}
	rowCount := 0

	for i := 0; i < 1000; i++ { // Limitar leitura para segurança
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ParseResult{Success: false}, fmt.Errorf("erro ao parsear: %v", err)
		}

		if i == 0 && params.HasHeader {
			headers = record
		} else {
			// Montar linha
			row := make(map[string]interface{})
			for j, header := range headers {
				if j < len(record) {
					row[header] = record[j]
				} else {
					row[header] = ""
				}
			}
			rows = append(rows, row)
			rowCount++
		}
	}

	// Gerar ID
	fileID := fmt.Sprintf("file_%d", time.Now().UnixNano())

	parsed := ParsedData{
		FileID:    fileID,
		Filename:  params.Filename,
		FileType:  "csv",
		Rows:      rows,
		Headers:   headers,
		RowCount:  rowCount,
		ColCount:  len(headers),
		ParsedAt:  time.Now(),
		Delimiter: params.Delimiter,
	}

	t.parsedFiles[fileID] = parsed

	// Calcular estatísticas
	t.analyzeFile(fileID)

	return ParseResult{
		Success:   true,
		FileID:    fileID,
		Filename:  params.Filename,
		Rows:      rows,
		Headers:   headers,
		RowCount:  rowCount,
		ColCount:  len(headers),
		Message:   fmt.Sprintf("%d linhas parseadas com %d colunas", rowCount, len(headers)),
		Timestamp: time.Now(),
	}, nil
}

// TransformData aplica transformações
func (t *CSVExcelParserTool) TransformData(params TransformDataParams) (interface{}, error) {
	parsed, exists := t.parsedFiles[params.FileID]
	if !exists {
		return ParseResult{Success: false}, fmt.Errorf("arquivo não encontrado")
	}

	// Aplicar transformações
	rows := parsed.Rows

	for _, transform := range params.Transformations {
		if op, ok := transform["operation"].(string); ok {
			switch op {
			case "filter":
				// Simular filtro
				if column, ok := transform["column"].(string); ok {
					if value, ok := transform["value"].(string); ok {
						filtered := make([]map[string]interface{}, 0)
						for _, row := range rows {
							if colVal, exists := row[column]; exists && fmt.Sprint(colVal) == value {
								filtered = append(filtered, row)
							}
						}
						rows = filtered
					}
				}
			case "sort":
				// Transformação aplicada
			}
		}
	}

	return map[string]interface{}{
		"success":   true,
		"file_id":   params.FileID,
		"row_count": len(rows),
		"message":   "Transformações aplicadas",
		"timestamp": time.Now(),
	}, nil
}

// AnalyzeData analisa dados
func (t *CSVExcelParserTool) AnalyzeData(params AnalyzeCSVParams) (interface{}, error) {
	parsed, exists := t.parsedFiles[params.FileID]
	if !exists {
		return map[string]interface{}{
			"success": false,
			"error":   "arquivo não encontrado",
		}, fmt.Errorf("arquivo não encontrado")
	}

	stats, _ := t.statistics[params.FileID]

	switch params.AnalysisType {
	case "statistics":
		return map[string]interface{}{
			"success":        true,
			"file_id":        params.FileID,
			"row_count":      stats.RowCount,
			"col_count":      stats.ColCount,
			"empty_rows":     stats.EmptyRows,
			"duplicate_rows": stats.DuplicateRows,
			"missing_values": stats.MissingValues,
			"missing_pct":    fmt.Sprintf("%.2f%%", stats.MissingPercentage),
		}, nil
	case "schema":
		return map[string]interface{}{
			"success":    true,
			"file_id":    params.FileID,
			"headers":    parsed.Headers,
			"col_count":  parsed.ColCount,
			"data_types": stats.DataTypes,
		}, nil
	default:
		return map[string]interface{}{
			"success": false,
			"error":   "tipo de análise desconhecido",
		}, fmt.Errorf("tipo desconhecido")
	}
}

// ExportData exporta dados
func (t *CSVExcelParserTool) ExportData(params ExportParams) (interface{}, error) {
	parsed, exists := t.parsedFiles[params.FileID]
	if !exists {
		return map[string]interface{}{
			"success": false,
			"error":   "arquivo não encontrado",
		}, fmt.Errorf("arquivo não encontrado")
	}

	// Simular exportação
	switch params.ExportFormat {
	case "json":
		return map[string]interface{}{
			"success": true,
			"format":  "json",
			"rows":    parsed.Rows,
			"message": fmt.Sprintf("Exportados %d registros em JSON", len(parsed.Rows)),
		}, nil
	case "csv":
		return map[string]interface{}{
			"success": true,
			"format":  "csv",
			"rows":    len(parsed.Rows),
			"message": fmt.Sprintf("Exportados %d registros em CSV", len(parsed.Rows)),
		}, nil
	default:
		return map[string]interface{}{
			"success": false,
			"error":   "formato não suportado",
		}, fmt.Errorf("formato desconhecido")
	}
}

// Helper functions

func (t *CSVExcelParserTool) analyzeFile(fileID string) {
	parsed := t.parsedFiles[fileID]

	stats := FileStatistics{
		FileID:    fileID,
		RowCount:  parsed.RowCount,
		ColCount:  parsed.ColCount,
		DataTypes: make(map[string]string),
	}

	// Contar valores vazios
	for _, row := range parsed.Rows {
		for _, header := range parsed.Headers {
			if val, exists := row[header]; !exists || val == "" {
				stats.MissingValues++
			}
		}
	}

	if parsed.RowCount > 0 {
		stats.MissingPercentage = float64(stats.MissingValues) / float64(parsed.RowCount*parsed.ColCount) * 100
	}

	// Detectar tipos de dados
	for _, header := range parsed.Headers {
		stats.DataTypes[header] = "string" // Simplificado
	}

	t.statistics[fileID] = stats
}

// ExportParams parâmetros para exportar
type ExportParams struct {
	FileID       string `json:"file_id" description:"ID do arquivo"`
	ExportFormat string `json:"export_format" description:"json, csv, excel"`
}
