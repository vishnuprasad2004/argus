package docker

import (
	dockerclient "github.com/docker/docker/client"
)

type DockerCollector struct {
	client *dockerclient.Client
}

type FetchOptions struct {
    TailLines string // "200", "500", "all"
    Since     string // "1h", "30m", "2024-01-15T10:00:00" — optional
}

// ContainerTarget holds info about one container shown in the picker menu
type ContainerTarget struct {
	ID     string
	Name   string
	Image  string
	Status string
}