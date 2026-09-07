package domain

import "time"

const RoutingScoreVersion = "health-v1"

type ProviderHealthSnapshot struct {
	ProviderConnectionID string
	WindowStart          time.Time
	WindowEnd            time.Time
	SampleCount          int
	QRSuccessCount       int
	ErrorCount           int
	TimeoutCount         int
	QRSuccessRate        float64
	ErrorRate            float64
	TimeoutRate          float64
	LatencyP50MS         float64
	LatencyP95MS         float64
	HealthScore          float64
	ScoreVersion         string
	ObservedAt           time.Time
}
