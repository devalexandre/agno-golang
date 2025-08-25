package v2

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// WorkflowExecutionInput represents the input data for a workflow execution
type WorkflowExecutionInput struct {
	Message        interface{}            `json:"message,omitempty"`
	AdditionalData map[string]interface{} `json:"additional_data,omitempty"`
	Images         []ImageArtifact        `json:"images,omitempty"`
	Videos         []VideoArtifact        `json:"videos,omitempty"`
	Audio          []AudioArtifact        `json:"audio,omitempty"`
}

// GetMessageAsString converts the message to a string representation
func (w *WorkflowExecutionInput) GetMessageAsString() string {
	if w.Message == nil {
		return ""
	}

	switch v := w.Message.(type) {
	case string:
		return v
	case map[string]interface{}, []interface{}:
		data, _ := json.MarshalIndent(v, "", "  ")
		return string(data)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ToMap converts WorkflowExecutionInput to a map
func (w *WorkflowExecutionInput) ToMap() map[string]interface{} {
	result := make(map[string]interface{})

	if w.Message != nil {
		result["message"] = w.Message
	}
	if w.AdditionalData != nil {
		result["additional_data"] = w.AdditionalData
	}
	if len(w.Images) > 0 {
		images := make([]map[string]interface{}, len(w.Images))
		for i, img := range w.Images {
			images[i] = img.ToMap()
		}
		result["images"] = images
	}
	if len(w.Videos) > 0 {
		videos := make([]map[string]interface{}, len(w.Videos))
		for i, vid := range w.Videos {
			videos[i] = vid.ToMap()
		}
		result["videos"] = videos
	}
	if len(w.Audio) > 0 {
		audio := make([]map[string]interface{}, len(w.Audio))
		for i, aud := range w.Audio {
			audio[i] = aud.ToMap()
		}
		result["audio"] = audio
	}

	return result
}

// StepInput represents the input data for a step execution
type StepInput struct {
	Message             interface{}            `json:"message,omitempty"`
	PreviousStepContent interface{}            `json:"previous_step_content,omitempty"`
	PreviousStepOutputs map[string]*StepOutput `json:"previous_step_outputs,omitempty"`
	AdditionalData      map[string]interface{} `json:"additional_data,omitempty"`
	Images              []ImageArtifact        `json:"images,omitempty"`
	Videos              []VideoArtifact        `json:"videos,omitempty"`
	Audio               []AudioArtifact        `json:"audio,omitempty"`
}

// GetMessageAsString converts the message to a string representation
func (s *StepInput) GetMessageAsString() string {
	if s.Message == nil {
		return ""
	}

	switch v := s.Message.(type) {
	case string:
		return v
	case map[string]interface{}, []interface{}:
		data, _ := json.MarshalIndent(v, "", "  ")
		return string(data)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// GetStepOutput gets output from a specific previous step by name
func (s *StepInput) GetStepOutput(stepName string) *StepOutput {
	if s.PreviousStepOutputs == nil {
		return nil
	}
	return s.PreviousStepOutputs[stepName]
}

// GetStepContent gets content from a specific previous step by name
func (s *StepInput) GetStepContent(stepName string) interface{} {
	stepOutput := s.GetStepOutput(stepName)
	if stepOutput == nil {
		return nil
	}

	// If this is a parallel step with sub-outputs, return structured map
	if stepOutput.ParallelStepOutputs != nil && len(stepOutput.ParallelStepOutputs) > 0 {
		result := make(map[string]interface{})
		for subStepName, subOutput := range stepOutput.ParallelStepOutputs {
			if subOutput.Content != nil {
				result[subStepName] = subOutput.Content
			}
		}
		return result
	}

	// Regular step, return content directly
	return stepOutput.Content
}

// GetAllPreviousContent gets concatenated content from all previous steps
func (s *StepInput) GetAllPreviousContent() string {
	if s.PreviousStepOutputs == nil || len(s.PreviousStepOutputs) == 0 {
		return ""
	}

	var contentParts []string
	for stepName, output := range s.PreviousStepOutputs {
		if output.Content != nil {
			contentParts = append(contentParts, fmt.Sprintf("=== %s ===\n%v", stepName, output.Content))
		}
	}

	if len(contentParts) == 0 {
		return ""
	}

	result := ""
	for i, part := range contentParts {
		if i > 0 {
			result += "\n\n"
		}
		result += part
	}
	return result
}

// GetLastStepContent gets content from the most recent step
func (s *StepInput) GetLastStepContent() interface{} {
	if s.PreviousStepOutputs == nil || len(s.PreviousStepOutputs) == 0 {
		return nil
	}

	// Get the last output (note: map iteration order is not guaranteed in Go)
	// In production, you might want to maintain order explicitly
	var lastOutput *StepOutput
	for _, output := range s.PreviousStepOutputs {
		lastOutput = output
	}

	if lastOutput != nil {
		return lastOutput.Content
	}
	return nil
}

// ToMap converts StepInput to a map
func (s *StepInput) ToMap() map[string]interface{} {
	result := make(map[string]interface{})

	if s.Message != nil {
		result["message"] = s.Message
	}
	if s.PreviousStepContent != nil {
		result["previous_step_content"] = s.PreviousStepContent
	}
	if s.PreviousStepOutputs != nil {
		outputs := make(map[string]interface{})
		for k, v := range s.PreviousStepOutputs {
			outputs[k] = v.ToMap()
		}
		result["previous_step_outputs"] = outputs
	}
	if s.AdditionalData != nil {
		result["additional_data"] = s.AdditionalData
	}
	if len(s.Images) > 0 {
		images := make([]map[string]interface{}, len(s.Images))
		for i, img := range s.Images {
			images[i] = img.ToMap()
		}
		result["images"] = images
	}
	if len(s.Videos) > 0 {
		videos := make([]map[string]interface{}, len(s.Videos))
		for i, vid := range s.Videos {
			videos[i] = vid.ToMap()
		}
		result["videos"] = videos
	}
	if len(s.Audio) > 0 {
		audio := make([]map[string]interface{}, len(s.Audio))
		for i, aud := range s.Audio {
			audio[i] = aud.ToMap()
		}
		result["audio"] = audio
	}

	return result
}

// StepOutput represents the output from a step execution
type StepOutput struct {
	Content             interface{}            `json:"content,omitempty"`
	StepName            string                 `json:"step_name,omitempty"`
	ExecutorName        string                 `json:"executor_name,omitempty"`
	ExecutorType        string                 `json:"executor_type,omitempty"`
	Event               string                 `json:"event,omitempty"`
	NextStep            string                 `json:"next_step,omitempty"`
	ParallelStepOutputs map[string]*StepOutput `json:"parallel_step_outputs,omitempty"`
	LoopStepOutputs     []*StepOutput          `json:"loop_step_outputs,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
	Images              []ImageArtifact        `json:"images,omitempty"`
	Videos              []VideoArtifact        `json:"videos,omitempty"`
	Audio               []AudioArtifact        `json:"audio,omitempty"`
	Metrics             *StepMetrics           `json:"metrics,omitempty"`
}

// ToMap converts StepOutput to a map
func (s *StepOutput) ToMap() map[string]interface{} {
	result := make(map[string]interface{})

	if s.Content != nil {
		result["content"] = s.Content
	}
	if s.StepName != "" {
		result["step_name"] = s.StepName
	}
	if s.ExecutorName != "" {
		result["executor_name"] = s.ExecutorName
	}
	if s.ExecutorType != "" {
		result["executor_type"] = s.ExecutorType
	}
	if s.Event != "" {
		result["event"] = s.Event
	}
	if s.NextStep != "" {
		result["next_step"] = s.NextStep
	}
	if s.ParallelStepOutputs != nil {
		outputs := make(map[string]interface{})
		for k, v := range s.ParallelStepOutputs {
			outputs[k] = v.ToMap()
		}
		result["parallel_step_outputs"] = outputs
	}
	if s.LoopStepOutputs != nil {
		outputs := make([]interface{}, len(s.LoopStepOutputs))
		for i, v := range s.LoopStepOutputs {
			outputs[i] = v.ToMap()
		}
		result["loop_step_outputs"] = outputs
	}
	if s.Metadata != nil {
		result["metadata"] = s.Metadata
	}
	if len(s.Images) > 0 {
		images := make([]map[string]interface{}, len(s.Images))
		for i, img := range s.Images {
			images[i] = img.ToMap()
		}
		result["images"] = images
	}
	if len(s.Videos) > 0 {
		videos := make([]map[string]interface{}, len(s.Videos))
		for i, vid := range s.Videos {
			videos[i] = vid.ToMap()
		}
		result["videos"] = videos
	}
	if len(s.Audio) > 0 {
		audio := make([]map[string]interface{}, len(s.Audio))
		for i, aud := range s.Audio {
			audio[i] = aud.ToMap()
		}
		result["audio"] = audio
	}
	if s.Metrics != nil {
		result["metrics"] = s.Metrics.ToMap()
	}

	return result
}

// StepMetrics represents metrics for a step execution
type StepMetrics struct {
	StartTime     time.Time              `json:"start_time,omitempty"`
	EndTime       time.Time              `json:"end_time,omitempty"`
	DurationMs    int64                  `json:"duration_ms,omitempty"`
	TokensUsed    int                    `json:"tokens_used,omitempty"`
	Cost          float64                `json:"cost,omitempty"`
	Success       bool                   `json:"success"`
	Error         string                 `json:"error,omitempty"`
	RetryCount    int                    `json:"retry_count,omitempty"`
	CustomMetrics map[string]interface{} `json:"custom_metrics,omitempty"`
}

// ToMap converts StepMetrics to a map
func (s *StepMetrics) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"start_time":     s.StartTime,
		"end_time":       s.EndTime,
		"duration_ms":    s.DurationMs,
		"tokens_used":    s.TokensUsed,
		"cost":           s.Cost,
		"success":        s.Success,
		"error":          s.Error,
		"retry_count":    s.RetryCount,
		"custom_metrics": s.CustomMetrics,
	}
}

// WorkflowMetrics represents metrics for a workflow execution
type WorkflowMetrics struct {
	WorkflowID     string                  `json:"workflow_id,omitempty"`
	RunID          string                  `json:"run_id,omitempty"`
	StartTime      time.Time               `json:"start_time,omitempty"`
	EndTime        time.Time               `json:"end_time,omitempty"`
	DurationMs     int64                   `json:"duration_ms,omitempty"`
	StepsExecuted  int                     `json:"steps_executed,omitempty"`
	StepsSucceeded int                     `json:"steps_succeeded,omitempty"`
	StepsFailed    int                     `json:"steps_failed,omitempty"`
	TotalTokens    int                     `json:"total_tokens,omitempty"`
	TotalCost      float64                 `json:"total_cost,omitempty"`
	Success        bool                    `json:"success"`
	Error          string                  `json:"error,omitempty"`
	StepMetrics    map[string]*StepMetrics `json:"step_metrics,omitempty"`
	CustomMetrics  map[string]interface{}  `json:"custom_metrics,omitempty"`
}

// ToMap converts WorkflowMetrics to a map
func (w *WorkflowMetrics) ToMap() map[string]interface{} {
	stepMetrics := make(map[string]interface{})
	if w.StepMetrics != nil {
		for k, v := range w.StepMetrics {
			stepMetrics[k] = v.ToMap()
		}
	}

	return map[string]interface{}{
		"workflow_id":     w.WorkflowID,
		"run_id":          w.RunID,
		"start_time":      w.StartTime,
		"end_time":        w.EndTime,
		"duration_ms":     w.DurationMs,
		"steps_executed":  w.StepsExecuted,
		"steps_succeeded": w.StepsSucceeded,
		"steps_failed":    w.StepsFailed,
		"total_tokens":    w.TotalTokens,
		"total_cost":      w.TotalCost,
		"success":         w.Success,
		"error":           w.Error,
		"step_metrics":    stepMetrics,
		"custom_metrics":  w.CustomMetrics,
	}
}

// Media Artifacts (placeholder types - should be imported from media package)
type ImageArtifact struct {
	URL         string                 `json:"url,omitempty"`
	Path        string                 `json:"path,omitempty"`
	Base64      string                 `json:"base64,omitempty"`
	ContentType string                 `json:"content_type,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

func (i ImageArtifact) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"url":          i.URL,
		"path":         i.Path,
		"base64":       i.Base64,
		"content_type": i.ContentType,
		"metadata":     i.Metadata,
	}
}

type VideoArtifact struct {
	URL         string                 `json:"url,omitempty"`
	Path        string                 `json:"path,omitempty"`
	ContentType string                 `json:"content_type,omitempty"`
	Duration    float64                `json:"duration,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

func (v VideoArtifact) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"url":          v.URL,
		"path":         v.Path,
		"content_type": v.ContentType,
		"duration":     v.Duration,
		"metadata":     v.Metadata,
	}
}

type AudioArtifact struct {
	URL         string                 `json:"url,omitempty"`
	Path        string                 `json:"path,omitempty"`
	ContentType string                 `json:"content_type,omitempty"`
	Duration    float64                `json:"duration,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

func (a AudioArtifact) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"url":          a.URL,
		"path":         a.Path,
		"content_type": a.ContentType,
		"duration":     a.Duration,
		"metadata":     a.Metadata,
	}
}

// ExecutorFunc represents a function that can be used as a step executor
type ExecutorFunc func(*StepInput) (*StepOutput, error)

// AsyncExecutorFunc represents an async function that can be used as a step executor
type AsyncExecutorFunc func(*StepInput) (<-chan *StepOutput, error)

// GenerateID generates a unique ID
func GenerateID() string {
	return uuid.New().String()
}
