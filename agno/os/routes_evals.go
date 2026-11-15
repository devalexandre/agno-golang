package os

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/devalexandre/agno-golang/agno/agent"
	agnoeval "github.com/devalexandre/agno-golang/agno/eval"
	"github.com/devalexandre/agno-golang/agno/models"
)

// Eval routes, ported from Python agno/os/routers/evals/evals.py.

var evalTypes = []string{"accuracy", "agent_as_judge", "performance", "reliability"}

func evalSchema(r *EvalRunRecord) gin.H {
	out := gin.H{
		"id":         r.RunID,
		"eval_type":  r.EvalType,
		"eval_data":  r.EvalData,
		"created_at": unixToTime(r.CreatedAt),
		"updated_at": unixToTime(r.UpdatedAt),
	}
	set := func(k, v string) {
		if v != "" {
			out[k] = v
		}
	}
	set("name", r.Name)
	set("agent_id", r.AgentID)
	set("team_id", r.TeamID)
	set("workflow_id", r.WorkflowID)
	set("model_id", r.ModelID)
	set("model_provider", r.ModelProvider)
	set("evaluated_component_name", r.EvaluatedComponentName)
	set("user_id", r.UserID)
	if r.EvalInput != nil {
		out["eval_input"] = r.EvalInput
	}
	return out
}

func (os *AgentOS) listEvalRunsHandler(c *gin.Context) {
	db, ok := os.dbFromRequest(c)
	if !ok {
		return
	}
	limit, ok := queryInt(c, "limit", 20, 1)
	if !ok {
		return
	}
	page, ok := queryInt(c, "page", 1, 0)
	if !ok {
		return
	}
	var types []string
	if raw := c.Query("eval_types"); raw != "" {
		for _, t := range strings.Split(raw, ",") {
			t = strings.TrimSpace(t)
			if !containsString(evalTypes, t) {
				httpError(c, http.StatusUnprocessableEntity, fmt.Sprintf("Invalid eval type: %s", t))
				return
			}
			types = append(types, t)
		}
	}
	filterType := c.Query("type")
	if filterType != "" && !containsString([]string{"agent", "team", "workflow"}, filterType) {
		httpError(c, http.StatusUnprocessableEntity, "Invalid filter type")
		return
	}
	runs, total, err := db.ListEvalRuns(c.Request.Context(), EvalQuery{
		AgentID: c.Query("agent_id"), TeamID: c.Query("team_id"), WorkflowID: c.Query("workflow_id"),
		ModelID: c.Query("model_id"), EvalTypes: types, FilterType: filterType,
		SortBy: c.DefaultQuery("sort_by", "created_at"), SortOrder: c.DefaultQuery("sort_order", "desc"),
		Limit: limit, Page: page,
	})
	if err != nil {
		httpError(c, http.StatusInternalServerError, err.Error())
		return
	}
	data := make([]gin.H, 0, len(runs))
	for _, r := range runs {
		data = append(data, evalSchema(r))
	}
	c.JSON(http.StatusOK, paginatedResponse(data, page, limit, total))
}

func (os *AgentOS) getEvalRunHandler(c *gin.Context) {
	db, ok := os.dbFromRequest(c)
	if !ok {
		return
	}
	id := c.Param("eval_run_id")
	r, err := db.GetEvalRun(c.Request.Context(), id)
	if err != nil {
		httpError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if r == nil {
		httpError(c, http.StatusNotFound, fmt.Sprintf("Eval run with id '%s' not found", id))
		return
	}
	c.JSON(http.StatusOK, evalSchema(r))
}

func (os *AgentOS) deleteEvalRunsHandler(c *gin.Context) {
	db, ok := os.dbFromRequest(c)
	if !ok {
		return
	}
	var req struct {
		EvalRunIDs []string `json:"eval_run_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.EvalRunIDs) == 0 {
		httpError(c, http.StatusUnprocessableEntity, "eval_run_ids must contain at least one id")
		return
	}
	if err := db.DeleteEvalRuns(c.Request.Context(), req.EvalRunIDs); err != nil {
		httpError(c, http.StatusInternalServerError, fmt.Sprintf("Failed to delete eval runs: %v", err))
		return
	}
	c.Status(http.StatusNoContent)
}

func (os *AgentOS) updateEvalRunHandler(c *gin.Context) {
	db, ok := os.dbFromRequest(c)
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Name) < 1 || len(req.Name) > 255 {
		httpError(c, http.StatusUnprocessableEntity, "name must be between 1 and 255 characters")
		return
	}
	id := c.Param("eval_run_id")
	r, err := db.GetEvalRun(c.Request.Context(), id)
	if err != nil {
		httpError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if r == nil {
		httpError(c, http.StatusNotFound, fmt.Sprintf("Eval run with id '%s' not found", id))
		return
	}
	r.Name = req.Name
	r.UpdatedAt = time.Now().Unix()
	if err := db.UpsertEvalRun(c.Request.Context(), r); err != nil {
		httpError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, evalSchema(r))
}

type evalRunInput struct {
	AgentID                   string                 `json:"agent_id"`
	TeamID                    string                 `json:"team_id"`
	ModelID                   string                 `json:"model_id"`
	ModelProvider             string                 `json:"model_provider"`
	EvalType                  string                 `json:"eval_type"`
	Input                     string                 `json:"input"`
	AdditionalGuidelines      string                 `json:"additional_guidelines"`
	AdditionalContext         string                 `json:"additional_context"`
	NumIterations             int                    `json:"num_iterations"`
	Name                      string                 `json:"name"`
	ExpectedOutput            string                 `json:"expected_output"`
	WarmupRuns                int                    `json:"warmup_runs"`
	ExpectedToolCalls         []string               `json:"expected_tool_calls"`
	AllowAdditionalToolCalls  bool                   `json:"allow_additional_tool_calls"`
	ExpectedToolCallArguments map[string]interface{} `json:"expected_tool_call_arguments"`
}

func toMapAny(v interface{}) map[string]interface{} {
	b, _ := json.Marshal(v)
	out := map[string]interface{}{}
	_ = json.Unmarshal(b, &out)
	return out
}

// createEvalRunHandler runs an evaluation against an agent and stores the
// result (POST /eval-runs). Team targets are not evaluable in the Go port
// (teams do not satisfy the eval AgentRunner contract).
func (os *AgentOS) createEvalRunHandler(c *gin.Context) {
	db, ok := os.dbFromRequest(c)
	if !ok {
		return
	}
	var in evalRunInput
	if err := c.ShouldBindJSON(&in); err != nil {
		httpError(c, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}
	if !containsString(evalTypes, in.EvalType) {
		httpError(c, http.StatusUnprocessableEntity, "eval_type must be one of accuracy, agent_as_judge, performance, reliability")
		return
	}
	if in.Input == "" {
		httpError(c, http.StatusUnprocessableEntity, "input is required")
		return
	}
	if in.NumIterations == 0 {
		in.NumIterations = 1
	}
	if in.NumIterations < 1 || in.NumIterations > 100 {
		httpError(c, http.StatusUnprocessableEntity, "num_iterations must be between 1 and 100")
		return
	}
	if in.AgentID != "" && in.TeamID != "" {
		httpError(c, http.StatusBadRequest, "Only one of agent_id or team_id must be provided")
		return
	}
	if in.AgentID == "" {
		if in.TeamID != "" {
			httpError(c, http.StatusBadRequest, "Team evaluations are not supported by the Go AgentOS")
			return
		}
		httpError(c, http.StatusBadRequest, "One of agent_id or team_id must be provided")
		return
	}
	target := os.findAgent(in.AgentID)
	if target == nil {
		httpError(c, http.StatusNotFound, fmt.Sprintf("Agent with id '%s' not found", in.AgentID))
		return
	}

	modelID, provider := GetAgentModelInfo(target)
	if in.ModelID != "" {
		modelID = in.ModelID
	}
	if in.ModelProvider != "" {
		provider = in.ModelProvider
	}
	var data map[string]interface{}
	var err error
	ctx := c.Request.Context()
	switch in.EvalType {
	case "accuracy":
		if in.ExpectedOutput == "" {
			httpError(c, http.StatusBadRequest, "expected_output is required for accuracy evaluation")
			return
		}
		e := &agnoeval.AccuracyEval{
			Name: in.Name, Agent: target, Input: in.Input, ExpectedOutput: in.ExpectedOutput,
			Judge: target.GetModel(), AdditionalContext: in.AdditionalContext, NumIterations: in.NumIterations,
		}
		if in.AdditionalGuidelines != "" {
			e.AdditionalGuidelines = []string{in.AdditionalGuidelines}
		}
		var res *agnoeval.AccuracyResult
		if res, err = e.Run(ctx); err == nil {
			data = toMapAny(res)
		}
	case "performance":
		e := agnoeval.NewPerformanceEval(in.Name, func() error {
			_, runErr := target.Run(in.Input)
			return runErr
		})
		e.NumIterations, e.WarmupRuns = in.NumIterations, in.WarmupRuns
		var res *agnoeval.PerformanceResult
		if res, err = e.Run(); err == nil {
			data = toMapAny(res)
		}
	case "reliability":
		var resp models.RunResponse
		if resp, err = target.Run(in.Input, agent.WithRunID(newUUID())); err == nil {
			e := &agnoeval.ReliabilityEval{
				Name: in.Name, AgentResponse: &resp, ExpectedToolCalls: in.ExpectedToolCalls,
				AllowAdditionalToolCalls: in.AllowAdditionalToolCalls,
			}
			if len(in.ExpectedToolCallArguments) > 0 {
				e.ExpectedToolCallArguments = map[string][]map[string]interface{}{}
				for tool, v := range in.ExpectedToolCallArguments {
					switch args := v.(type) {
					case map[string]interface{}:
						e.ExpectedToolCallArguments[tool] = []map[string]interface{}{args}
					case []interface{}:
						for _, a := range args {
							if m, ok := a.(map[string]interface{}); ok {
								e.ExpectedToolCallArguments[tool] = append(e.ExpectedToolCallArguments[tool], m)
							}
						}
					}
				}
			}
			var res *agnoeval.ReliabilityResult
			if res, err = e.Run(); err == nil {
				data = toMapAny(res)
			}
		}
	default:
		httpError(c, http.StatusBadRequest, "agent_as_judge evaluations are not supported by the Go AgentOS")
		return
	}
	if err != nil {
		httpError(c, http.StatusInternalServerError, fmt.Sprintf("Failed to run evaluation: %v", err))
		return
	}
	now := time.Now().Unix()
	userID := ""
	if auth := requestAuthorization(c); auth != nil {
		userID = auth.Context.UserID
	}
	rec := &EvalRunRecord{
		RunID: newUUID(), Name: in.Name, EvalType: in.EvalType, AgentID: in.AgentID,
		ModelID: modelID, ModelProvider: provider, EvaluatedComponentName: target.GetName(), UserID: userID,
		EvalData: data, EvalInput: toMapAny(in), CreatedAt: now, UpdatedAt: now,
	}
	if err := db.UpsertEvalRun(ctx, rec); err != nil {
		httpError(c, http.StatusInternalServerError, fmt.Sprintf("Failed to store evaluation: %v", err))
		return
	}
	c.JSON(http.StatusOK, evalSchema(rec))
}
