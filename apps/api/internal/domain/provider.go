package domain

import "time"

type Environment string

const (
	EnvironmentTest Environment = "test"
	EnvironmentLive Environment = "live"
)

type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"
	CircuitOpen     CircuitState = "open"
	CircuitHalfOpen CircuitState = "half_open"
)

type ProviderConnection struct {
	ID               string
	WorkspaceID      string
	ProviderKey      string
	Environment      Environment
	Enabled          bool
	CredentialsValid bool
	Circuit          CircuitState
	CircuitOpenedAt  *time.Time
	Priority         int
}
