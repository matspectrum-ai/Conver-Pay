package domain

import "time"

type CandidateSnapshot struct {
	ProviderConnectionID string
	ProviderKey          string
	Priority             int
	Eligible             bool
	ExclusionReason      string
}

type RoutingDecision struct {
	ID                           string
	PaymentIntentID              string
	AttemptID                    string
	SelectedProviderConnectionID string
	Candidates                   []CandidateSnapshot
	ReasonCodes                  []string
	CreatedAt                    time.Time
}
