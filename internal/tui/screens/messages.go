package screens

import (
    "github.com/vishnuprasad2004/argus/internal/collectors/docker"
    "github.com/tmc/langchaingo/llms/googleai"
)

type SwitchToSourceSelect struct{}

type SwitchToContainerSelect struct {
    Source string
}

// SwitchToChat — removed Collector interface{}, use concrete types
type SwitchToChat struct {
    Target docker.ContainerTarget   // the selected container
    LLM    *googleai.GoogleAI       // passed through so chat screen can use it
}