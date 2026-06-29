package docker

import (
	"context"
	"fmt"
	"io"
	"bufio"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/vishnuprasad2004/argus/agents"
)

// FetchLogs pulls the last N lines from a container — works even if stopped
// This is the "what happened?" mode
func (d *DockerCollector) FetchLogs(ctx context.Context, target ContainerTarget, opts FetchOptions) ([]agents.LogEntry, error) {
    reader, err := d.client.ContainerLogs(ctx, target.ID, container.LogsOptions{
        ShowStdout: true,
        ShowStderr: true,
        Follow:     false,          // false = fetch snapshot, don't tail
        Tail:       opts.TailLines, // line count cap
        Since:      opts.Since,     // time filter, empty string = ignored by Docker
        Timestamps: true,
    })
    if err != nil {
        return nil, fmt.Errorf("fetch logs: %w", err)
    }
    defer reader.Close()

    pr, pw := io.Pipe()
    go func() {
        stdcopy.StdCopy(pw, pw, reader)
        pw.Close()
    }()

    var logs []agents.LogEntry
    scanner := bufio.NewScanner(pr)
    for scanner.Scan() {
        logs = append(logs, parseDockerLine(scanner.Text(), target))
    }
    return logs, scanner.Err()
}