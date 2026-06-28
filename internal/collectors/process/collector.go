package process

import (
	"io"
	"os"
	"fmt"
	"time"
	"bufio"
	"context"
	"strings"
	"os/exec"
	"path/filepath"
	"github.com/vishnuprasad2004/argus/agents"
)

// NewProcessCollector sets up the collector in the user's current directory
// workingDir = wherever user ran "argus" from = their project root
func NewProcessCollector() (*ProcessCollector, error) {
	// os.Getwd() = get current working directory, like process.cwd() in Node
	workingDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("cannot get working directory: %w", err)
	}

	// create .argus-session.log in project root
	// os.Create truncates if exists — fresh log each session
	logPath := filepath.Join(workingDir, ".argus-session.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("cannot create log file: %w", err)
	}

	fmt.Printf("✓ Session logs will be saved to: %s\n", logPath)

	return &ProcessCollector{
		workingDir: workingDir,
		logFile:    logFile,
	}, nil
}

// Start takes the user's command string, splits it, and runs it
// "npm run dev"  → exec("npm", "run", "dev")
// "python main.py" → exec("python", "main.py")
// "go run ."    → exec("go", "run", ".")
func (p *ProcessCollector) Start(
	ctx context.Context,
	command string, // raw command string from user
) (<-chan agents.LogEntry, <-chan ProcessResult, error) {

	// split "npm run dev" into ["npm", "run", "dev"]
	// strings.Fields splits on any whitespace, like split(/\s+/) in JS
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil, nil, fmt.Errorf("empty command")
	}

	// parts[0] = "npm", parts[1:] = ["run", "dev"]
	// exec.CommandContext respects ctx — if ctx cancelled, process is killed
	p.cmd = exec.CommandContext(ctx, parts[0], parts[1:]...)
	p.cmd.Dir = p.workingDir // run in project directory, not argus binary dir

	// get stdout pipe — like Java's process.getInputStream()
	stdout, err := p.cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("stdout pipe: %w", err)
	}

	// get stderr pipe separately — this is how we detect errors visually
	stderr, err := p.cmd.StderrPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("stderr pipe: %w", err)
	}

	// start the process — non-blocking, like ProcessBuilder.start() in Java
	if err := p.cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("failed to start '%s': %w", command, err)
	}

	fmt.Printf("✓ Process started (PID %d): %s\n", p.cmd.Process.Pid, command)

	logCh := make(chan agents.LogEntry, 100)
	resultCh := make(chan ProcessResult, 1) // capacity 1 — only one exit event

	// pipe stdout and stderr into logCh concurrently
	// each runs in its own goroutine
	go p.pipeStream(stdout, "INFO", logCh)  // stdout = INFO (white)
	go p.pipeStream(stderr, "ERROR", logCh) // stderr = ERROR (red)

	// wait for process to exit in background
	// when it does, send ProcessResult to resultCh
	go p.waitForExit(resultCh, logCh)

	return logCh, resultCh, nil
}

// pipeStream reads from one pipe (stdout or stderr) line by line
// defaultLevel = "INFO" for stdout, "ERROR" for stderr
func (p *ProcessCollector) pipeStream(
	pipe io.ReadCloser,
	defaultLevel string,
	logCh chan<- agents.LogEntry, // chan<- means write-only from this function
) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		line := scanner.Text()

		entry := agents.LogEntry{
			Timestamp: time.Now(),
			Level:     defaultLevel, // stderr always ERROR, stdout checked below
			Source:    "process",
			Message:   line,
			Metadata: map[string]string{
				"working_dir": p.workingDir,
			},
		}

		// for stdout lines, still detect if they contain error keywords
		// example: some apps write "ERROR: db connection failed" to stdout
		if defaultLevel == "INFO" {
			upper := strings.ToUpper(line)
			switch {
			case strings.Contains(upper, "ERROR") ||
				strings.Contains(upper, "FATAL") ||
				strings.Contains(upper, "EXCEPTION") ||
				strings.Contains(upper, "PANIC"):
				entry.Level = "ERROR"
			case strings.Contains(upper, "WARN"):
				entry.Level = "WARN"
			}
		}

		// save to log file — write raw line + newline
		// ignore write errors here, don't crash the stream over a log write
		if p.logFile != nil {
			fmt.Fprintf(p.logFile, "[%s][%s] %s\n",
				entry.Timestamp.Format("15:04:05"),
				entry.Level,
				line,
			)
		}

		logCh <- entry
	}
}

// waitForExit blocks until the process exits, then sends result
// this is what triggers "show stats + open query bar" in TUI
func (p *ProcessCollector) waitForExit(
	resultCh chan<- ProcessResult,
	logCh chan agents.LogEntry,
) {
	// cmd.Wait() blocks until process exits — like process.waitFor() in Java
	err := p.cmd.Wait()

	// close logCh AFTER wait — ensures all pipe data is flushed first
	// goroutines reading pipes will finish naturally when pipes close
	// small sleep lets pipeStream goroutines drain remaining lines
	time.Sleep(100 * time.Millisecond)
	close(logCh)

	// close log file
	if p.logFile != nil {
		p.logFile.Close()
	}

	if err != nil {
		// process crashed or was killed
		resultCh <- ProcessResult{
			ExitCode: p.cmd.ProcessState.ExitCode(),
			Err:      err,
			Message:  fmt.Sprintf("Process crashed (exit code %d): %v", p.cmd.ProcessState.ExitCode(), err),
		}
	} else {
		// clean exit (exit code 0)
		resultCh <- ProcessResult{
			ExitCode: 0,
			Err:      nil,
			Message:  "Process exited cleanly (code 0)",
		}
	}
}


// Kill stops the process — called when user presses ctrl+c in TUI
func (p *ProcessCollector) Kill() error {
    if p.cmd != nil && p.cmd.Process != nil {
        return p.cmd.Process.Kill()
    }
    return nil
}

// LogFilePath returns the path so TUI can show it to user
func (p *ProcessCollector) LogFilePath() string {
    return filepath.Join(p.workingDir, ".argus-session.log")
}