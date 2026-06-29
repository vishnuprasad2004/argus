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
