package orchestration

import (
	"sort"

	"github.com/matspectrum-ai/conver-pay/apps/api/internal/domain"
)

type Router struct{}

type RouteResult struct {
	Selected   *domain.ProviderConnection
	Candidates []domain.CandidateSnapshot
}

func (Router) Select(connections []domain.ProviderConnection, env domain.Environment, excluded map[string]bool) RouteResult {
	ordered := append([]domain.ProviderConnection(nil), connections...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Priority != ordered[j].Priority {
			return ordered[i].Priority < ordered[j].Priority
		}
		return ordered[i].ID < ordered[j].ID
	})

	result := RouteResult{Candidates: make([]domain.CandidateSnapshot, 0, len(ordered))}
	for i := range ordered {
		conn := ordered[i]
		snapshot := domain.CandidateSnapshot{
			ProviderConnectionID: conn.ID,
			ProviderKey:          conn.ProviderKey,
			Priority:             conn.Priority,
			Eligible:             true,
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
		if result.Selected == nil && snapshot.Eligible {
			selected := conn
			result.Selected = &selected
		}
	}

	return result
}
