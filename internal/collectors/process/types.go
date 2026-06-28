package process

import (
	"os"
	"os/exec"
)

type ProcessCollector struct {
	cmd        *exec.Cmd
	workingDir string   // where argus was launched from (project root)
	logFile    *os.File // .argus-session.log in project dir
}

// ProcessResult is sent when process exits — triggers stats + query bar in TUI
type ProcessResult struct {
	ExitCode int
	Err      error  // nil = clean exit, non-nil = crash
	Message  string // "Process exited (code 0)" or "Process crashed: signal SIGTERM"
}
