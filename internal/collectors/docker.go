package collectors

// collect docker logs from a container and return them as a slice of LogEntry structs
import (
	"context"
	"fmt"
	"io"

	"bufio"
	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/vishnuprasad2004/argus/agents"
	"strings"
	"time"
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

// Stream tails logs from one container and sends them as LogEntry
// returns two channels:
//
//	logCh — one LogEntry per log line
//	errCh — any error including "container stopped"
//
// caller does: logCh, errCh := collector.Stream(ctx, target)
func (d *DockerCollector) Stream(ctx context.Context, target ContainerTarget) (<-chan agents.LogEntry, <-chan error) {
	// buffered channels — like BlockingQueue(100) in Java
	// buffered means sender won't block if receiver is slightly slow
	logCh := make(chan agents.LogEntry, 100)
	errCh := make(chan error, 1)

	// run in background goroutine — like new Thread().start() in Java
	// Stream() returns immediately, logs flow in background
	go func() {
		defer close(logCh) // always close channels when goroutine exits
		defer close(errCh) // this signals caller that stream ended

		// ask Docker for log stream — like getting an InputStream
		reader, err := d.client.ContainerLogs(ctx, target.ID, container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true, // keep streaming (like tail -f)
			Tail:       "50", // start with last 50 lines, then live
			Timestamps: true, // include timestamps in each line
		})
		if err != nil {
			errCh <- fmt.Errorf("container logs: %w", err)
			return
		}
		defer reader.Close()

		// IMPORTANT: Docker multiplexes stdout+stderr with an 8-byte binary
		// header on each line. Without stdcopy, you get garbage characters.
		// io.Pipe() = Java's PipedInputStream/PipedOutputStream
		pr, pw := io.Pipe()
		go func() {
			// StdCopy strips the headers and writes clean text to pw
			stdcopy.StdCopy(pw, pw, reader)
			pw.Close()
		}()

		// read line by line — like BufferedReader.readLine() in Java
		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				// context cancelled — user quit or timeout
				errCh <- fmt.Errorf("stream cancelled for %s", target.Name)
				return
			default:
				// normal path — parse line and send to channel
				entry := parseDockerLine(scanner.Text(), target)
				logCh <- entry
			}
		}

		// scanner.Scan() returned false — stream ended
		// check why: error or clean container exit
		if err := scanner.Err(); err != nil {
			errCh <- fmt.Errorf("scanner: %w", err)
		} else {
			errCh <- fmt.Errorf("container %s stopped", target.Name)
		}
	}()

	return logCh, errCh
}

// parseDockerLine turns one raw docker log line into a clean LogEntry
// raw format from docker: "2024-01-15T10:23:45.123456789Z some message here"
func parseDockerLine(raw string, target ContainerTarget) agents.LogEntry {
	entry := agents.LogEntry{
		Source:    target.Name,
		Timestamp: time.Now(),
		Level:     "INFO",
		Message:   raw, // default: use whole line as message
		Metadata: map[string]string{
			"container_id": target.ID,
			"image":        target.Image,
		},
	}

	// Strategy 1: Docker timestamp prefix
	// format: "2024-01-15T10:23:45.123Z rest of message"
	parts := strings.SplitN(raw, " ", 2)
	if len(parts) == 2 {
		t, err := time.Parse(time.RFC3339Nano, parts[0])
		if err == nil {
			entry.Timestamp = t
			entry.Message = parts[1]
			raw = parts[1] // update raw so level detection below uses clean message
		}
	}

	// Strategy 2: detect level from common unstructured patterns
	upper := strings.ToUpper(raw)
	switch {
	case strings.Contains(upper, "FATAL") ||
		strings.Contains(upper, "PANIC") ||
		strings.Contains(upper, "NPM ERROR") || // npm error lines
		strings.Contains(upper, "UNHANDLEDREJECTION"):
		entry.Level = "ERROR"

	case strings.Contains(upper, "ERROR") ||
		strings.Contains(upper, "ERR!") || // node style
		strings.Contains(upper, "EXCEPTION") ||
		strings.Contains(upper, "SIGNAL SIGTERM"): // process killed
		entry.Level = "ERROR"

	case strings.Contains(upper, "WARN") ||
		strings.Contains(upper, "WARNING") ||
		strings.Contains(upper, "DEPRECATED"):
		entry.Level = "WARN"

	case strings.Contains(upper, "DEBUG") ||
		strings.Contains(upper, "VERBOSE"):
		entry.Level = "DEBUG"
	}

	// Strategy 3: stack trace lines — keep as ERROR, group them
	// lines starting with "at " are JS/Java stack frames
	if strings.HasPrefix(strings.TrimSpace(raw), "at ") {
		entry.Level = "ERROR" // stack trace = belongs to an error
		entry.Metadata["is_stack_trace"] = "true"
	}

	return entry
}

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