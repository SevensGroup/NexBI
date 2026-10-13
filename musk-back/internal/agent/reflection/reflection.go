package reflection

import (
	"context"
	"strings"

	"autonomous-agent/internal/agent/decision"
)

const (
	// NextContinue keeps the decision loop running after reflection.
	NextContinue = "CONTINUE"
	// NextReplan rebuilds the task DAG after reflection.
	NextReplan = "REPLAN"
	// NextFinish ends the run with a final answer.
	NextFinish = "FINISH"
	// NextAskUser pauses for a user question.
	NextAskUser = "ASK_USER"

	// ConsecutiveRepeatLimit is how many identical decision/tool/argument
	// triples in a row force REPLAN.
	ConsecutiveRepeatLimit = 3
)

// Reflection is the structured judgment after a tool observation.
type Reflection struct {
	GoalSatisfied      bool     `json:"goal_satisfied" jsonschema:"required" jsonschema_description:"Whether the user goal is complete"`
	EvidenceSufficient bool     `json:"evidence_sufficient" jsonschema:"required" jsonschema_description:"Whether observations contain enough evidence"`
	ResultCorrect      bool     `json:"result_correct" jsonschema:"required" jsonschema_description:"Whether tool results look complete and correct"`
	MissingInformation []string `json:"missing_information,omitempty" jsonschema_description:"Information still needed"`
	Errors             []string `json:"errors,omitempty" jsonschema_description:"Anomalies or tool errors"`
	NextAction         string   `json:"next_action" jsonschema:"required,enum=CONTINUE,enum=REPLAN,enum=FINISH,enum=ASK_USER" jsonschema_description:"Next loop action"`
	Confidence         float64  `json:"confidence" jsonschema:"required,minimum=0,maximum=1" jsonschema_description:"Confidence in [0, 1]"`
}

// Reflector is the Reflection Engine contract used by the runtime loop.
type Reflector interface {
	Reflect(ctx context.Context, state *decision.AgentState, observation decision.Observation) (*Reflection, error)
}

// View copies a Reflection onto AgentState working memory.
func (r *Reflection) View() decision.ReflectionView {
	if r == nil {
		return decision.ReflectionView{}
	}
	return decision.ReflectionView{
		GoalSatisfied:      r.GoalSatisfied,
		EvidenceSufficient: r.EvidenceSufficient,
		ResultCorrect:      r.ResultCorrect,
		MissingInformation: append([]string(nil), r.MissingInformation...),
		Errors:             append([]string(nil), r.Errors...),
		NextAction:         r.NextAction,
		Confidence:         r.Confidence,
	}
}

func normalize(r *Reflection) *Reflection {
	if r == nil {
		r = &Reflection{}
	}
	r.NextAction = strings.ToUpper(strings.TrimSpace(r.NextAction))
	if r.MissingInformation == nil {
		r.MissingInformation = []string{}
	}
	if r.Errors == nil {
		r.Errors = []string{}
	}
	return r
}
