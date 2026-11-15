package team

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/google/uuid"
)

// ErrTeamPausedRunNotFound is returned by Team.ContinueRun for an unknown run.
var ErrTeamPausedRunNotFound = errors.New("team paused run not found")

// teamPausedRun is where a team run stopped because a member paused.
type teamPausedRun struct {
	RunID           string
	Prompt          string
	Mode            TeamMode
	MemberIndex     int
	MemberResponses []string
	Model           string
	MemberRun       models.RunResponse
}

type teamHITL struct {
	mu   sync.Mutex
	runs map[string]*teamPausedRun
}

type memberPause struct {
	state      *teamPausedRun
	memberResp models.RunResponse
}

// runMembersFrom runs members sequentially from index start, stopping at the
// first member whose run pauses.
func (t *Team) runMembersFrom(prompt string, start int, memberResponses []string) ([]string, *memberPause) {
	for i := start; i < len(t.members); i++ {
		member := t.members[i]
		memberResp, err := member.Run(prompt)
		if err != nil {
			if t.debug {
				memberResponses = append(memberResponses, fmt.Sprintf("Member %d (%s) error: %v", i+1, member.GetName(), err))
			}
			continue
		}
		if memberResp.Status == models.RunStatusPaused {
			return memberResponses, &memberPause{
				state: &teamPausedRun{
					Prompt:          prompt,
					MemberIndex:     i,
					MemberResponses: append([]string(nil), memberResponses...),
				},
				memberResp: memberResp,
			}
		}
		memberResponses = t.appendMemberResponse(memberResponses, member, memberResp)
	}
	return memberResponses, nil
}

func (t *Team) appendMemberResponse(responses []string, member TeamMember, resp models.RunResponse) []string {
	if t.showMembersResponses {
		return append(responses, fmt.Sprintf("**%s Response:**\n%s", member.GetName(), resp.TextContent))
	}
	return append(responses, resp.TextContent)
}

func routeResponse(memberResponse models.RunResponse, model string) models.RunResponse {
	return models.RunResponse{
		TextContent: memberResponse.TextContent,
		ContentType: memberResponse.ContentType,
		Event:       "TeamRouteResponse",
		Messages:    memberResponse.Messages,
		Model:       model,
		CreatedAt:   time.Now().Unix(),
	}
}

// pauseForMember records the paused team run and returns the PAUSED team
// response carrying the member's requirements (tagged with member context).
func (t *Team) pauseForMember(st *teamPausedRun, memberResp models.RunResponse) models.RunResponse {
	if st.RunID == "" {
		st.RunID = uuid.New().String()
	}
	st.MemberRun = memberResp
	member := t.members[st.MemberIndex]
	for _, req := range memberResp.Requirements {
		req.MemberAgentName = member.GetName()
		req.MemberRunID = memberResp.RunID
		if aw, ok := member.(*AgentWrapper); ok {
			req.MemberAgentID = aw.agent.GetID()
		}
	}
	t.hitl.mu.Lock()
	if t.hitl.runs == nil {
		t.hitl.runs = map[string]*teamPausedRun{}
	}
	t.hitl.runs[st.RunID] = st
	t.hitl.mu.Unlock()

	return models.RunResponse{
		TextContent:    memberResp.TextContent,
		ContentType:    "text",
		Event:          models.RunEventPaused,
		Status:         models.RunStatusPaused,
		RunID:          st.RunID,
		SessionID:      t.sessionID,
		Requirements:   memberResp.Requirements,
		ToolExecutions: memberResp.ToolExecutions,
		CreatedAt:      time.Now().Unix(),
	}
}

// ContinueRun resumes a team run paused by a member. run is the team run ID
// or the paused team RunResponse; resolve its Requirements first (or pass the
// resolved requirements). The member's run is continued with
// agent.ContinueRun, then the remaining members run and the team synthesizes
// as usual. The team run may pause again.
//
// Paused team state is kept in memory (the member's own paused state is
// persisted in its session when the member agent has a DB).
func (t *Team) ContinueRun(ctx context.Context, run interface{}, requirements ...*models.RunRequirement) (models.RunResponse, error) {
	var runID string
	var given []*models.RunRequirement
	switch v := run.(type) {
	case string:
		runID = v
	case *models.RunResponse:
		if v == nil {
			return models.RunResponse{}, errors.New("ContinueRun: nil run response")
		}
		runID, given = v.RunID, v.Requirements
	case models.RunResponse:
		runID, given = v.RunID, v.Requirements
	default:
		return models.RunResponse{}, fmt.Errorf("ContinueRun: unsupported run type %T", run)
	}
	given = append(given, requirements...)

	t.hitl.mu.Lock()
	st, ok := t.hitl.runs[runID]
	t.hitl.mu.Unlock()
	if !ok {
		return models.RunResponse{}, fmt.Errorf("%w: %s", ErrTeamPausedRunNotFound, runID)
	}
	member := t.members[st.MemberIndex]
	aw, ok := member.(*AgentWrapper)
	if !ok {
		return models.RunResponse{}, fmt.Errorf("member %s cannot be continued", member.GetName())
	}

	memberRun := st.MemberRun
	memberResp, err := aw.agent.ContinueRun(ctx, &memberRun, given...)
	if err != nil {
		return memberResp, err
	}
	if memberResp.Status == models.RunStatusPaused {
		return t.pauseForMember(st, memberResp), nil
	}

	t.hitl.mu.Lock()
	delete(t.hitl.runs, runID)
	t.hitl.mu.Unlock()

	var response models.RunResponse
	switch st.Mode {
	case RouteMode:
		response = routeResponse(memberResp, st.Model)
	default:
		responses := t.appendMemberResponse(st.MemberResponses, member, memberResp)
		responses, pause := t.runMembersFrom(st.Prompt, st.MemberIndex+1, responses)
		if pause != nil {
			pause.state.Mode = st.Mode
			pause.state.RunID = runID
			return t.pauseForMember(pause.state, pause.memberResp), nil
		}
		if st.Mode == CollaborateMode {
			response, err = t.finishCollaborate(st.Prompt, responses)
		} else {
			response, err = t.finishCoordinate(st.Prompt, responses)
		}
		if err != nil {
			response.Status = models.RunStatusError
			return response, err
		}
	}
	response.RunID = runID
	response.Status = models.RunStatusCompleted
	if t.memory != nil || t.storage != nil {
		t.saveToMemory(st.Prompt, response)
	}
	return response, nil
}
