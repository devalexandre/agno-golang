package tools

import (
	"fmt"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// MonitoringAlertsTool gerencia monitoramento e alertas
type MonitoringAlertsTool struct {
	toolkit.Toolkit
	metrics       []MetricPoint
	alerts        []AlertRule
	activeAlerts  []ActiveAlert
	events        []MonitoringEvent
	thresholds    map[string]float64
	maxDataPoints int
}

// MetricPoint representa um ponto de métrica
type MetricPoint struct {
	MetricID   string
	MetricName string
	Value      float64
	Timestamp  time.Time
	Tags       map[string]string
	Unit       string
}

// AlertRule representa uma regra de alerta
type AlertRule struct {
	AlertID    string
	Name       string
	Condition  string // "above", "below", "equal", "between"
	Threshold  float64
	Severity   string // "critical", "warning", "info"
	MetricName string
	Enabled    bool
	CreatedAt  time.Time
	NotifyTo   []string
}

// ActiveAlert representa um alerta ativo
type ActiveAlert struct {
	AlertInstanceID string
	AlertRuleID     string
	MetricValue     float64
	Status          string // "triggered", "acknowledged", "resolved"
	TriggeredAt     time.Time
	AcknowledgedAt  *time.Time
	ResolvedAt      *time.Time
}

// MonitoringEvent registra um evento de monitoramento
type MonitoringEvent struct {
	EventID   string
	Type      string // "metric_received", "alert_triggered", "alert_resolved"
	Severity  string
	Message   string
	Timestamp time.Time
}

// RecordMetricParams parâmetros para registrar métrica
type RecordMetricParams struct {
	MetricName string            `json:"metric_name" description:"Nome da métrica"`
	Value      float64           `json:"value" description:"Valor da métrica"`
	Unit       string            `json:"unit" description:"Unidade de medida"`
	Tags       map[string]string `json:"tags" description:"Tags associadas"`
}

// CreateAlertParams parâmetros para criar alerta
type CreateAlertParams struct {
	AlertName  string   `json:"alert_name" description:"Nome do alerta"`
	MetricName string   `json:"metric_name" description:"Nome da métrica"`
	Condition  string   `json:"condition" description:"above, below, equal, between"`
	Threshold  float64  `json:"threshold" description:"Valor limite"`
	Severity   string   `json:"severity" description:"critical, warning, info"`
	NotifyTo   []string `json:"notify_to" description:"Contatos para notificação"`
}

// GetMetricsParams parâmetros para obter métricas
type GetMetricsParams struct {
	MetricName string `json:"metric_name" description:"Nome da métrica"`
	TimeRange  int    `json:"time_range" description:"Range em minutos"`
}

// AcknowledgeAlertParams parâmetros para reconhecer alerta
type AcknowledgeAlertParams struct {
	AlertInstanceID string `json:"alert_instance_id" description:"ID da instância do alerta"`
}

// NewMonitoringAlertsTool cria nova instância
func NewMonitoringAlertsTool() *MonitoringAlertsTool {
	tool := &MonitoringAlertsTool{
		metrics:       make([]MetricPoint, 0),
		alerts:        make([]AlertRule, 0),
		activeAlerts:  make([]ActiveAlert, 0),
		events:        make([]MonitoringEvent, 0),
		thresholds:    make(map[string]float64),
		maxDataPoints: 10000,
	}
	tool.Toolkit = toolkit.NewToolkit()
	tool.Toolkit.Name = "MonitoringAlertsTool"
	tool.Toolkit.Description = "Ferramenta para monitoramento e gerenciamento de alertas"

	tool.Register("record_metric",
		"Registrar uma métrica",
		tool,
		tool.RecordMetric,
		RecordMetricParams{},
	)

	tool.Register("create_alert",
		"Criar nova regra de alerta",
		tool,
		tool.CreateAlert,
		CreateAlertParams{},
	)

	tool.Register("get_metrics",
		"Obter métricas registradas",
		tool,
		tool.GetMetrics,
		GetMetricsParams{},
	)

	tool.Register("get_active_alerts",
		"Obter alertas ativos",
		tool,
		tool.GetActiveAlerts,
		struct{}{},
	)

	tool.Register("acknowledge_alert",
		"Reconhecer um alerta",
		tool,
		tool.AcknowledgeAlert,
		AcknowledgeAlertParams{},
	)

	tool.Register("list_alert_rules",
		"Listar regras de alerta",
		tool,
		tool.ListAlertRules,
		struct{}{},
	)

	tool.Register("get_monitoring_events",
		"Obter eventos de monitoramento",
		tool,
		tool.GetMonitoringEvents,
		struct {
			Limit int `json:"limit"`
		}{},
	)

	tool.Register("delete_alert_rule",
		"Deletar regra de alerta",
		tool,
		tool.DeleteAlertRule,
		struct {
			AlertID string `json:"alert_id"`
		}{},
	)

	return tool
}

// RecordMetric registra uma métrica
func (t *MonitoringAlertsTool) RecordMetric(params RecordMetricParams) (map[string]interface{}, error) {
	if params.MetricName == "" {
		return nil, fmt.Errorf("metric_name não pode estar vazio")
	}

	metricID := fmt.Sprintf("metric_%d", time.Now().UnixNano())

	metric := MetricPoint{
		MetricID:   metricID,
		MetricName: params.MetricName,
		Value:      params.Value,
		Timestamp:  time.Now(),
		Tags:       params.Tags,
		Unit:       params.Unit,
	}

	t.metrics = append(t.metrics, metric)

	event := MonitoringEvent{
		EventID:   fmt.Sprintf("ev_%d", time.Now().UnixNano()),
		Type:      "metric_received",
		Severity:  "info",
		Message:   fmt.Sprintf("Métrica %s registrada: %.2f", params.MetricName, params.Value),
		Timestamp: time.Now(),
	}
	t.events = append(t.events, event)

	// Verificar alertas
	t.evaluateAlerts(params.MetricName, params.Value)

	return map[string]interface{}{
		"success":     true,
		"metric_id":   metricID,
		"metric_name": params.MetricName,
		"value":       params.Value,
		"unit":        params.Unit,
		"timestamp":   time.Now().Format(time.RFC3339),
	}, nil
}

// CreateAlert cria uma nova regra de alerta
func (t *MonitoringAlertsTool) CreateAlert(params CreateAlertParams) (map[string]interface{}, error) {
	if params.AlertName == "" {
		return nil, fmt.Errorf("alert_name não pode estar vazio")
	}

	if params.MetricName == "" {
		return nil, fmt.Errorf("metric_name não pode estar vazio")
	}

	alertID := fmt.Sprintf("alert_%d", time.Now().UnixNano())

	alert := AlertRule{
		AlertID:    alertID,
		Name:       params.AlertName,
		Condition:  params.Condition,
		Threshold:  params.Threshold,
		Severity:   params.Severity,
		MetricName: params.MetricName,
		Enabled:    true,
		CreatedAt:  time.Now(),
		NotifyTo:   params.NotifyTo,
	}

	t.alerts = append(t.alerts, alert)

	event := MonitoringEvent{
		EventID:   fmt.Sprintf("ev_%d", time.Now().UnixNano()),
		Type:      "alert_created",
		Severity:  "info",
		Message:   fmt.Sprintf("Alerta criado: %s", params.AlertName),
		Timestamp: time.Now(),
	}
	t.events = append(t.events, event)

	return map[string]interface{}{
		"success":   true,
		"alert_id":  alertID,
		"name":      params.AlertName,
		"condition": params.Condition,
		"threshold": params.Threshold,
		"severity":  params.Severity,
		"enabled":   true,
	}, nil
}

// GetMetrics obtém métricas registradas
func (t *MonitoringAlertsTool) GetMetrics(params GetMetricsParams) (map[string]interface{}, error) {
	if params.MetricName == "" {
		return nil, fmt.Errorf("metric_name não pode estar vazio")
	}

	startTime := time.Now().Add(-time.Duration(params.TimeRange) * time.Minute)
	metrics := make([]map[string]interface{}, 0)

	for _, m := range t.metrics {
		if m.MetricName == params.MetricName && m.Timestamp.After(startTime) {
			metrics = append(metrics, map[string]interface{}{
				"metric_id": m.MetricID,
				"value":     m.Value,
				"unit":      m.Unit,
				"timestamp": m.Timestamp.Format(time.RFC3339),
				"tags":      m.Tags,
			})
		}
	}

	avg := 0.0
	max := 0.0
	min := 0.0

	if len(metrics) > 0 {
		var sum float64
		for i, m := range metrics {
			val := m["value"].(float64)
			sum += val
			if i == 0 {
				max = val
				min = val
			} else {
				if val > max {
					max = val
				}
				if val < min {
					min = val
				}
			}
		}
		avg = sum / float64(len(metrics))
	}

	return map[string]interface{}{
		"success":     true,
		"metric_name": params.MetricName,
		"count":       len(metrics),
		"average":     fmt.Sprintf("%.2f", avg),
		"max":         fmt.Sprintf("%.2f", max),
		"min":         fmt.Sprintf("%.2f", min),
		"time_range":  fmt.Sprintf("%d minutes", params.TimeRange),
		"metrics":     metrics,
	}, nil
}

// GetActiveAlerts obtém alertas ativos
func (t *MonitoringAlertsTool) GetActiveAlerts(params struct{}) (map[string]interface{}, error) {
	activeAlerts := make([]map[string]interface{}, 0)

	for _, alert := range t.activeAlerts {
		if alert.Status == "triggered" {
			activeAlerts = append(activeAlerts, map[string]interface{}{
				"alert_instance_id": alert.AlertInstanceID,
				"alert_rule_id":     alert.AlertRuleID,
				"metric_value":      alert.MetricValue,
				"status":            alert.Status,
				"triggered_at":      alert.TriggeredAt.Format(time.RFC3339),
			})
		}
	}

	return map[string]interface{}{
		"success":       true,
		"total":         len(activeAlerts),
		"active_alerts": activeAlerts,
	}, nil
}

// AcknowledgeAlert reconhece um alerta
func (t *MonitoringAlertsTool) AcknowledgeAlert(params AcknowledgeAlertParams) (map[string]interface{}, error) {
	if params.AlertInstanceID == "" {
		return nil, fmt.Errorf("alert_instance_id não pode estar vazio")
	}

	for i, alert := range t.activeAlerts {
		if alert.AlertInstanceID == params.AlertInstanceID {
			now := time.Now()
			t.activeAlerts[i].AcknowledgedAt = &now
			t.activeAlerts[i].Status = "acknowledged"
			break
		}
	}

	return map[string]interface{}{
		"success":           true,
		"alert_instance_id": params.AlertInstanceID,
		"status":            "acknowledged",
		"acknowledged_at":   time.Now().Format(time.RFC3339),
	}, nil
}

// ListAlertRules lista regras de alerta
func (t *MonitoringAlertsTool) ListAlertRules(params struct{}) (map[string]interface{}, error) {
	rules := make([]map[string]interface{}, 0)

	for _, alert := range t.alerts {
		rules = append(rules, map[string]interface{}{
			"alert_id":   alert.AlertID,
			"name":       alert.Name,
			"metric":     alert.MetricName,
			"condition":  alert.Condition,
			"threshold":  alert.Threshold,
			"severity":   alert.Severity,
			"enabled":    alert.Enabled,
			"created_at": alert.CreatedAt.Format(time.RFC3339),
		})
	}

	return map[string]interface{}{
		"success": true,
		"total":   len(rules),
		"rules":   rules,
	}, nil
}

// GetMonitoringEvents obtém eventos de monitoramento
func (t *MonitoringAlertsTool) GetMonitoringEvents(params struct{ Limit int }) (map[string]interface{}, error) {
	limit := params.Limit
	if limit == 0 || limit > len(t.events) {
		limit = len(t.events)
	}

	events := make([]map[string]interface{}, 0)

	// Pegar os últimos eventos (ordem reversa)
	start := len(t.events) - limit
	if start < 0 {
		start = 0
	}

	for i := start; i < len(t.events); i++ {
		ev := t.events[i]
		events = append(events, map[string]interface{}{
			"event_id":  ev.EventID,
			"type":      ev.Type,
			"severity":  ev.Severity,
			"message":   ev.Message,
			"timestamp": ev.Timestamp.Format(time.RFC3339),
		})
	}

	return map[string]interface{}{
		"success": true,
		"total":   len(events),
		"events":  events,
	}, nil
}

// DeleteAlertRule deleta uma regra de alerta
func (t *MonitoringAlertsTool) DeleteAlertRule(params struct{ AlertID string }) (map[string]interface{}, error) {
	if params.AlertID == "" {
		return nil, fmt.Errorf("alert_id não pode estar vazio")
	}

	for i, alert := range t.alerts {
		if alert.AlertID == params.AlertID {
			t.alerts = append(t.alerts[:i], t.alerts[i+1:]...)
			break
		}
	}

	return map[string]interface{}{
		"success":  true,
		"alert_id": params.AlertID,
		"status":   "deleted",
	}, nil
}

// evaluateAlerts avalia se uma métrica dispara alertas
func (t *MonitoringAlertsTool) evaluateAlerts(metricName string, value float64) {
	for _, alert := range t.alerts {
		if alert.Enabled && alert.MetricName == metricName {
			shouldTrigger := false

			switch alert.Condition {
			case "above":
				shouldTrigger = value > alert.Threshold
			case "below":
				shouldTrigger = value < alert.Threshold
			case "equal":
				shouldTrigger = value == alert.Threshold
			}

			if shouldTrigger {
				// Criar instância de alerta ativo
				activeAlert := ActiveAlert{
					AlertInstanceID: fmt.Sprintf("ai_%d", time.Now().UnixNano()),
					AlertRuleID:     alert.AlertID,
					MetricValue:     value,
					Status:          "triggered",
					TriggeredAt:     time.Now(),
				}

				t.activeAlerts = append(t.activeAlerts, activeAlert)

				event := MonitoringEvent{
					EventID:   fmt.Sprintf("ev_%d", time.Now().UnixNano()),
					Type:      "alert_triggered",
					Severity:  alert.Severity,
					Message:   fmt.Sprintf("Alerta %s disparado: %s = %.2f", alert.Name, metricName, value),
					Timestamp: time.Now(),
				}
				t.events = append(t.events, event)
			}
		}
	}
}
