package orchestration

import (
	"sort"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
)

type Router struct{}

type RouteResult struct {
	Selected     *domain.ProviderConnection
	Candidates   []domain.CandidateSnapshot
	ReasonCodes  []string
	ScoreVersion string
}

func (Router) Select(connections []domain.ProviderConnection, env domain.Environment, excluded map[string]bool, health ...map[string]domain.ProviderHealthSnapshot) RouteResult {
	healthByConnection := map[string]domain.ProviderHealthSnapshot{}
	if len(health) > 0 && health[0] != nil {
		healthByConnection = health[0]
	}

	ordered := append([]domain.ProviderConnection(nil), connections...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Priority != ordered[j].Priority {
			return ordered[i].Priority < ordered[j].Priority
		}
		return ordered[i].ID < ordered[j].ID
	})

	result := RouteResult{
		Candidates:   make([]domain.CandidateSnapshot, 0, len(ordered)),
		ReasonCodes:  []string{"priority_cold_start"},
		ScoreVersion: "priority-v1",
	}
	if len(healthByConnection) > 0 {
		result.ReasonCodes = []string{"automatic_health_score"}
		result.ScoreVersion = domain.RoutingScoreVersion
	}

	var best *domain.ProviderConnection
	for i := range ordered {
		conn := ordered[i]
		snapshot := domain.CandidateSnapshot{
			ProviderConnectionID: conn.ID,
			ProviderKey:          conn.ProviderKey,
			Priority:             conn.Priority,
			Eligible:             true,
			Score:                0.5,
			ScoreVersion:         "cold-start-v1",
		}
		if h, ok := healthByConnection[conn.ID]; ok {
			snapshot.Score = h.HealthScore
			snapshot.ScoreVersion = h.ScoreVersion
			snapshot.SampleCount = h.SampleCount
			snapshot.QRSuccessRate = h.QRSuccessRate
			snapshot.ErrorRate = h.ErrorRate
			snapshot.TimeoutRate = h.TimeoutRate
			snapshot.LatencyP95MS = h.LatencyP95MS
		}

		switch {
		case excluded[conn.ID]:
			snapshot.Eligible = false
			snapshot.ExclusionReason = "already_attempted"
		case !conn.Enabled:
			snapshot.Eligible = false
			snapshot.ExclusionReason = "disabled"
		case conn.Environment != env:
			snapshot.Eligible = false
			snapshot.ExclusionReason = "wrong_environment"
		case !conn.CredentialsValid:
			snapshot.Eligible = false
			snapshot.ExclusionReason = "invalid_credentials"
		case conn.Circuit == domain.CircuitOpen:
			snapshot.Eligible = false
			snapshot.ExclusionReason = "circuit_open"
		}

		result.Candidates = append(result.Candidates, snapshot)
		if snapshot.Eligible && (best == nil || betterCandidate(conn, *best, healthByConnection)) {
			copy := conn
			best = &copy
		}
	}
	result.Selected = best
	return result
}

func betterCandidate(candidate, current domain.ProviderConnection, health map[string]domain.ProviderHealthSnapshot) bool {
	candidateRank := circuitRank(candidate.Circuit)
	currentRank := circuitRank(current.Circuit)
	if candidateRank != currentRank {
		return candidateRank < currentRank
	}
	candidateValue, candidateHasHealth := candidateScore(candidate.ID, health)
	currentValue, currentHasHealth := candidateScore(current.ID, health)
	if candidateHasHealth || currentHasHealth {
		if candidateValue != currentValue {
			return candidateValue > currentValue
		}
	}
	if candidate.Priority != current.Priority {
		return candidate.Priority < current.Priority
	}
	return candidate.ID < current.ID
}

func candidateScore(connectionID string, health map[string]domain.ProviderHealthSnapshot) (float64, bool) {
	snapshot, ok := health[connectionID]
	if !ok {
		return 0.5, false
	}
	return snapshot.HealthScore, true
}

func circuitRank(state domain.CircuitState) int {
	switch state {
	case domain.CircuitClosed:
		return 0
	case domain.CircuitHalfOpen:
		return 1
	default:
		return 2
	}
}
