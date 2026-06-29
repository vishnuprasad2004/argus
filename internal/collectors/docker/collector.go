package docker

import (
	"context"
	"fmt"
	"strings"
	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
)

func NewDockerCollector() (*DockerCollector, error) {
	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,                     // reads DOCKER_HOST automatically
		dockerclient.WithAPIVersionNegotiation(), // handles version differences
	)
	if err != nil {
		return nil, fmt.Errorf("docker collector: %w", err)
	}
	return &DockerCollector{client: cli}, nil
}

// Ping checks if Docker daemon is running before we do anything
func (d *DockerCollector) Validate(ctx context.Context) error {
	_, err := d.client.Ping(ctx)
	if err != nil {
		return fmt.Errorf("docker daemon not reachable: %w", err)
	}
	return nil
}

// ListContainers returns all currently running containers
// This feeds the TUI selector menu later
func (d *DockerCollector) ListContainers(ctx context.Context) ([]ContainerTarget, error) {
	containers, err := d.client.ContainerList(ctx, container.ListOptions{
		All: true, // only running, skip stopped ones
	})
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	result := make([]ContainerTarget, len(containers))
	for i, c := range containers {
		result[i] = ContainerTarget{
			ID:     c.ID[:12],                           // short ID like docker ps
			Name:   strings.TrimPrefix(c.Names[0], "/"), // docker adds "/" prefix
			Image:  c.Image,
			Status: c.Status,
		}
	}
	return result, nil
}

