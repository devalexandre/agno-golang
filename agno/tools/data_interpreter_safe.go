package tools

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DataInterpreterSafe interpreta dados de forma segura
type DataInterpreterSafe struct {
	toolkit.Toolkit
	analysisCache map[string]AnalysisResult
	dataLimits    DataLimits
}

// DataLimits limites de segurança para análise
type DataLimits struct {
	MaxRows              int
	MaxColumns           int
	MaxStringLength      int
	MaxExecutionTimeMs   int64
	AllowedOperations    []string
}

// AnalysisResult resultado de análise
type AnalysisResult struct {
	AnalysisID      string                 `json:"analysis_id"`
	DataShape       DataShape              `json:"data_shape"`
	Summary         DataSummary            `json:"summary"`
	Statistics      map[string]interface{} `json:"statistics"`
	Insights        []string               `json:"insights"`
	Anomalies       []string               `json:"anomalies"`
	ExecutionTime   int64                  `json:"execution_time_ms"`
	CreatedAt       time.Time              `json:"created_at"`
	IsValid         bool                   `json:"is_valid"`
	Warnings        []string               `json:"warnings"`
}

// DataShape forma dos dados
type DataShape struct {
	Rows       int      `json:"rows"`
	Columns    int      `json:"columns"`
	ColumnNames []string `json:"column_names"`
	ColumnTypes []string `json:"column_types"`
}

// DataSummary resumo dos dados
type DataSummary struct {
	TotalRows      int                    `json:"total_rows"`
	CompleteRows   int                    `json:"complete_rows"`
	MissingValues  map[string]int         `json:"missing_values"`
	DuplicateRows  int                    `json:"duplicate_rows"`
	ColumnStats    map[string]ColumnStat  `json:"column_stats"`
}

// ColumnStat estatísticas de coluna
type ColumnStat struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"` // numeric, text, date, boolean
	NonNull     int         `json:"non_null"`
	Unique      int         `json:"unique"`
	Mean        float64     `json:"mean,omitempty"`
	Median      float64     `json:"median,omitempty"`
	StdDev      float64     `json:"std_dev,omitempty"`
	Min         interface{} `json:"min,omitempty"`
	Max         interface{} `json:"max,omitempty"`
	MostCommon  interface{} `json:"most_common,omitempty"`
}

// AnalyzeDataParams parâmetros para analisar dados
type AnalyzeDataParams struct {
	Data          []map[string]interface{} `json:"data" description:"Dados em formato de mapa"`
	IncludeStats  bool                     `json:"include_stats" description:"Calcular estatísticas"`
	DetectAnomalies bool                   `json:"detect_anomalies" description:"Detectar anomalias"`
	SampleSize    int                      `json:"sample_size" description:"Tamanho de amostra (0 = todas)"`
}

// ValidateDataParams parâmetros para validar dados
type ValidateDataParams struct {
	Data           []map[string]interface{} `json:"data" description:"Dados a validar"`
	Schema         map[string]string        `json:"schema" description:"Schema esperado (name -> type)"`
	StrictMode     bool                     `json:"strict_mode" description:"Modo rigoroso"`
}

// AggregateDataParams parâmetros para agregar dados
type AggregateDataParams struct {
	Data       []map[string]interface{} `json:"data" description:"Dados a agregar"`
	GroupBy    []string                 `json:"group_by" description:"Colunas para agrupar"`
	Operations []AggregateOp            `json:"operations" description:"Operações de agregação"`
}

// AggregateOp operação de agregação
type AggregateOp struct {
	Column    string `json:"column" description:"Coluna"`
	Operation string `json:"operation" description:"sum, count, mean, min, max"`
	AsName    string `json:"as_name" description:"Nome do resultado"`
}

// InterpretationResult resultado de interpretação
type InterpretationResult struct {
	Success     bool                   `json:"success"`
	Message     string                 `json:"message"`
	Data        interface{}            `json:"data,omitempty"`
	Analysis    AnalysisResult         `json:"analysis,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
}

// NewDataInterpreterSafe cria novo interpretador
func NewDataInterpreterSafe() *DataInterpreterSafe {
	d := &DataInterpreterSafe{
		analysisCache: make(map[string]AnalysisResult),
		dataLimits: DataLimits{
			MaxRows:            100000,
			MaxColumns:         1000,
			MaxStringLength:    10000,
			MaxExecutionTimeMs: 5000,
			AllowedOperations:  []string{"analyze", "validate", "aggregate", "filter", "transform"},
		},
	}
	d.Toolkit = toolkit.NewToolkit()

	d.Toolkit.Register(
		"AnalyzeData",
		"Analisar dados e gerar estatísticas",
		d,
		d.AnalyzeData,
		AnalyzeDataParams{},
	)

	d.Toolkit.Register(
		"ValidateData",
		"Validar dados contra schema",
		d,
		d.ValidateData,
		ValidateDataParams{},
	)

	d.Toolkit.Register(
		"AggregateData",
		"Agregar dados por grupos",
		d,
		d.AggregateData,
		AggregateDataParams{},
	)

	d.Toolkit.Register(
		"GetDataInsights",
		"Obter insights de dados",
		d,
		d.GetDataInsights,
		GetInsightsParams{},
	)

	d.Toolkit.Register(
		"DetectOutliers",
		"Detectar outliers nos dados",
		d,
		d.DetectOutliers,
		DetectOutliersParams{},
	)

	return d
}

// AnalyzeData analisa dados
func (d *DataInterpreterSafe) AnalyzeData(params AnalyzeDataParams) (interface{}, error) {
	startTime := time.Now()

	if len(params.Data) == 0 {
		return InterpretationResult{Success: false}, fmt.Errorf("dados vazios")
	}

	// Validar limites de segurança
	if len(params.Data) > d.dataLimits.MaxRows {
		return InterpretationResult{Success: false}, fmt.Errorf("dados excedem limite de %d linhas", d.dataLimits.MaxRows)
	}

	// Determinar schema
	schema := d.inferSchema(params.Data)

	dataShape := DataShape{
		Rows:        len(params.Data),
		Columns:     len(schema),
		ColumnNames: make([]string, 0),
		ColumnTypes: make([]string, 0),
	}

	for colName, colType := range schema {
		dataShape.ColumnNames = append(dataShape.ColumnNames, colName)
		dataShape.ColumnTypes = append(dataShape.ColumnTypes, colType)
	}

	// Calcular resumo
	summary := d.calculateSummary(params.Data, schema)

	// Calcular estatísticas
	stats := make(map[string]interface{})
	if params.IncludeStats {
		stats = d.calculateStatistics(params.Data, schema)
	}

	// Detectar anomalias
	anomalies := make([]string, 0)
	if params.DetectAnomalies {
		anomalies = d.detectAnomalies(params.Data, schema)
	}

	analysisID := fmt.Sprintf("analysis_%d", time.Now().UnixNano())

	result := AnalysisResult{
		AnalysisID:    analysisID,
		DataShape:     dataShape,
		Summary:       summary,
		Statistics:    stats,
		Anomalies:     anomalies,
		ExecutionTime: time.Since(startTime).Milliseconds(),
		CreatedAt:     time.Now(),
		IsValid:       true,
		Warnings:      make([]string, 0),
	}

	d.analysisCache[analysisID] = result

	return InterpretationResult{
		Success:   true,
		Message:   fmt.Sprintf("Dados analisados: %d linhas, %d colunas", len(params.Data), len(schema)),
		Analysis:  result,
		Timestamp: time.Now(),
	}, nil
}

// ValidateData valida dados contra schema
func (d *DataInterpreterSafe) ValidateData(params ValidateDataParams) (interface{}, error) {
	if len(params.Data) == 0 {
		return InterpretationResult{Success: false}, fmt.Errorf("dados vazios")
	}

	violations := make([]string, 0)

	for i, row := range params.Data {
		for colName, expectedType := range params.Schema {
			value, exists := row[colName]
			if !exists {
				violations = append(violations, fmt.Sprintf("Linha %d: coluna '%s' ausente", i+1, colName))
				continue
			}

			// Validar tipo
			actualType := d.getType(value)
			if params.StrictMode && actualType != expectedType {
				violations = append(violations, fmt.Sprintf("Linha %d: coluna '%s' tipo inválido (esperado %s, encontrado %s)", i+1, colName, expectedType, actualType))
			}
		}
	}

	isValid := len(violations) == 0

	return InterpretationResult{
		Success: isValid,
		Message: fmt.Sprintf("Validação: %d erros encontrados", len(violations)),
		Data: map[string]interface{}{
			"is_valid":   isValid,
			"violations": violations,
			"total":      len(params.Data),
		},
		Timestamp: time.Now(),
	}, nil
}

// AggregateData agrega dados
func (d *DataInterpreterSafe) AggregateData(params AggregateDataParams) (interface{}, error) {
	if len(params.Data) == 0 {
		return InterpretationResult{Success: false}, fmt.Errorf("dados vazios")
	}

	// Agrupar
	groups := make(map[string][]map[string]interface{})

	for _, row := range params.Data {
		groupKey := d.buildGroupKey(row, params.GroupBy)
		groups[groupKey] = append(groups[groupKey], row)
	}

	// Agregar
	result := make([]map[string]interface{}, 0)

	for groupKey, groupRows := range groups {
		agg := d.parseGroupKey(groupKey, params.GroupBy)

		for _, op := range params.Operations {
			value := d.performAggregation(groupRows, op)
			agg[op.AsName] = value
		}

		result = append(result, agg)
	}

	return InterpretationResult{
		Success:   true,
		Message:   fmt.Sprintf("Dados agregados em %d grupos", len(result)),
		Data:      result,
		Timestamp: time.Now(),
	}, nil
}

// GetDataInsights obtém insights dos dados
func (d *DataInterpreterSafe) GetDataInsights(params GetInsightsParams) (interface{}, error) {
	analysis, exists := d.analysisCache[params.AnalysisID]
	if !exists {
		return nil, fmt.Errorf("análise não encontrada")
	}

	insights := []string{
		fmt.Sprintf("Dataset contém %d linhas e %d colunas", analysis.DataShape.Rows, analysis.DataShape.Columns),
		fmt.Sprintf("Linhas completas: %d (%.1f%%)", analysis.Summary.CompleteRows, float64(analysis.Summary.CompleteRows)/float64(analysis.DataShape.Rows)*100),
	}

	insights = append(insights, analysis.Anomalies...)

	return map[string]interface{}{
		"analysis_id": params.AnalysisID,
		"insights":    insights,
		"count":       len(insights),
		"timestamp":   time.Now(),
	}, nil
}

// DetectOutliers detecta outliers
func (d *DataInterpreterSafe) DetectOutliers(params DetectOutliersParams) (interface{}, error) {
	if len(params.Data) == 0 {
		return nil, fmt.Errorf("dados vazios")
	}

	outliers := make([]map[string]interface{}, 0)

	// Método IQR simples
	schema := d.inferSchema(params.Data)

	for colName, colType := range schema {
		if colType != "numeric" {
			continue
		}

		values := make([]float64, 0)
		for _, row := range params.Data {
			if val, ok := row[colName]; ok {
				if fval, err := toFloat64(val); err == nil {
					values = append(values, fval)
				}
			}
		}

		if len(values) < 4 {
			continue
		}

		// Calcular quartis
		sort.Float64s(values)
		q1 := values[len(values)/4]
		q3 := values[3*len(values)/4]
		iqr := q3 - q1

		lowerBound := q1 - 1.5*iqr
		upperBound := q3 + 1.5*iqr

		for i, row := range params.Data {
			if val, ok := row[colName]; ok {
				if fval, err := toFloat64(val); err == nil {
					if fval < lowerBound || fval > upperBound {
						outliers = append(outliers, map[string]interface{}{
							"row":    i + 1,
							"column": colName,
							"value":  fval,
							"type":   "outlier",
						})
					}
				}
			}
		}
	}

	return map[string]interface{}{
		"success":      true,
		"outliers":     outliers,
		"count":        len(outliers),
		"timestamp":    time.Now(),
	}, nil
}

// Helper functions

func (d *DataInterpreterSafe) inferSchema(data []map[string]interface{}) map[string]string {
	schema := make(map[string]string)

	if len(data) == 0 {
		return schema
	}

	firstRow := data[0]
	for colName, value := range firstRow {
		schema[colName] = d.getType(value)
	}

	return schema
}

func (d *DataInterpreterSafe) getType(value interface{}) string {
	switch value.(type) {
	case float64, float32, int, int64, int32:
		return "numeric"
	case bool:
		return "boolean"
	case string:
		return "text"
	case time.Time:
		return "date"
	default:
		return "unknown"
	}
}

func (d *DataInterpreterSafe) calculateSummary(data []map[string]interface{}, schema map[string]string) DataSummary {
	summary := DataSummary{
		TotalRows:     len(data),
		MissingValues: make(map[string]int),
		ColumnStats:   make(map[string]ColumnStat),
	}

	completeRows := 0
	for _, row := range data {
		complete := true
		for colName := range schema {
			if _, exists := row[colName]; !exists {
				summary.MissingValues[colName]++
				complete = false
			}
		}
		if complete {
			completeRows++
		}
	}

	summary.CompleteRows = completeRows

	return summary
}

func (d *DataInterpreterSafe) calculateStatistics(data []map[string]interface{}, schema map[string]string) map[string]interface{} {
	stats := make(map[string]interface{})

	for colName, colType := range schema {
		if colType == "numeric" {
			values := make([]float64, 0)
			for _, row := range data {
				if val, ok := row[colName]; ok {
					if fval, err := toFloat64(val); err == nil {
						values = append(values, fval)
					}
				}
			}

			if len(values) > 0 {
				stats[colName] = map[string]interface{}{
					"count":  len(values),
					"mean":   calculateMean(values),
					"median": calculateMedian(values),
					"min":    calculateMin(values),
					"max":    calculateMax(values),
				}
			}
		}
	}

	return stats
}

func (d *DataInterpreterSafe) detectAnomalies(data []map[string]interface{}, schema map[string]string) []string {
	anomalies := make([]string, 0)

	if len(data) == 0 {
		return anomalies
	}

	// Verificar linhas duplicadas
	seen := make(map[string]int)
	for _, row := range data {
		key := fmt.Sprint(row)
		seen[key]++
	}

	duplicates := 0
	for _, count := range seen {
		if count > 1 {
			duplicates++
		}
	}

	if duplicates > 0 {
		anomalies = append(anomalies, fmt.Sprintf("Encontradas %d linhas potencialmente duplicadas", duplicates))
	}

	return anomalies
}

func (d *DataInterpreterSafe) buildGroupKey(row map[string]interface{}, groupBy []string) string {
	parts := make([]string, 0)
	for _, col := range groupBy {
		if val, ok := row[col]; ok {
			parts = append(parts, fmt.Sprint(val))
		}
	}
	return strings.Join(parts, "|")
}

func (d *DataInterpreterSafe) parseGroupKey(key string, groupBy []string) map[string]interface{} {
	result := make(map[string]interface{})
	parts := strings.Split(key, "|")
	for i, col := range groupBy {
		if i < len(parts) {
			result[col] = parts[i]
		}
	}
	return result
}

func (d *DataInterpreterSafe) performAggregation(rows []map[string]interface{}, op AggregateOp) interface{} {
	switch op.Operation {
	case "count":
		return len(rows)
	case "sum":
		sum := 0.0
		for _, row := range rows {
			if val, ok := row[op.Column]; ok {
				if fval, err := toFloat64(val); err == nil {
					sum += fval
				}
			}
		}
		return sum
	case "mean":
		return calculateMean(d.extractFloats(rows, op.Column))
	case "min":
		return calculateMin(d.extractFloats(rows, op.Column))
	case "max":
		return calculateMax(d.extractFloats(rows, op.Column))
	default:
		return nil
	}
}

func (d *DataInterpreterSafe) extractFloats(rows []map[string]interface{}, col string) []float64 {
	result := make([]float64, 0)
	for _, row := range rows {
		if val, ok := row[col]; ok {
			if fval, err := toFloat64(val); err == nil {
				result = append(result, fval)
			}
		}
	}
	return result
}

// Funções matemáticas simples

func toFloat64(v interface{}) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case float32:
		return float64(val), nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case string:
		return strconv.ParseFloat(val, 64)
	default:
		return 0, fmt.Errorf("cannot convert to float64")
	}
}

func calculateMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func calculateMedian(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sort.Float64s(values)
	if len(values)%2 == 0 {
		return (values[len(values)/2-1] + values[len(values)/2]) / 2
	}
	return values[len(values)/2]
}

func calculateMin(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	min := values[0]
	for _, v := range values {
		if v < min {
			min = v
		}
	}
	return min
}

func calculateMax(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	max := values[0]
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	return max
}

func calculateStdDev(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	mean := calculateMean(values)
	sumSq := 0.0
	for _, v := range values {
		sumSq += math.Pow(v-mean, 2)
	}
	variance := sumSq / float64(len(values))
	return math.Sqrt(variance)
}

// GetInsightsParams parâmetros para obter insights
type GetInsightsParams struct {
	AnalysisID string `json:"analysis_id" description:"ID da análise"`
}

// DetectOutliersParams parâmetros para detectar outliers
type DetectOutliersParams struct {
	Data       []map[string]interface{} `json:"data" description:"Dados a analisar"`
	Method     string                   `json:"method" description:"iqr, zscore"`
	Threshold  float64                  `json:"threshold" description:"Threshold para detecção"`
}
