package agents

import (
	"context"
	"fmt"
)

type StatsAgent struct{}

func (a *StatsAgent) Name() string { return "stats_agent" }

func (a *StatsAgent) Description() string {
	return "Counts log levels, computes error rate, returns metrics summary."
}

func (a *StatsAgent) Run(ctx context.Context, in AgentInput) (AgentOutput, error) {
	counts := map[string]int{
		"ERROR": 0,
		"WARN":  0,
		"INFO":  0,
		"DEBUG": 0,
	}

	for _, entry := range in.Logs {
		// if level exists in map, increment — otherwise ignore
		if _, ok := counts[entry.Level]; ok {
			counts[entry.Level]++
		}
	}

	total := len(in.Logs)
	errorRate := 0.0
	if total > 0 {
		// float64() = casting to double in Java
		errorRate = float64(counts["ERROR"]) / float64(total) * 100
	}

	result := fmt.Sprintf(
		"Total: %d lines | ERROR: %d | WARN: %d | INFO: %d | Error rate: %.1f%%",
		total, counts["ERROR"], counts["WARN"], counts["INFO"], errorRate,
	)

	return AgentOutput{
		Result: result,
		// Structured is for TUI charts later
		Structured: map[string]any{
			"total":      total,
			"counts":     counts,
			"error_rate": errorRate,
		},
	}, nil
}
