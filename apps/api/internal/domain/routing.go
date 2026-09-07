package domain

import "time"

type CandidateSnapshot struct {
	ProviderConnectionID string
	ProviderKey          string
	Priority             int
	Eligible             bool
	ExclusionReason      string
	Score                float64
	ScoreVersion         string
	SampleCount          int
	QRSuccessRate        float64
	ErrorRate            float64
	TimeoutRate          float64
	LatencyP95MS         float64
}

type RoutingDecision struct {
	ID                           string
	PaymentIntentID              string
	AttemptID                    string
	SelectedProviderConnectionID string
	Candidates                   []CandidateSnapshot
	ReasonCodes                  []string
	ScoreVersion                 string
	CreatedAt                    time.Time
}
