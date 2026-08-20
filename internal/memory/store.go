package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Store manages all memory files under ~/.argus/
type Store struct {
	baseDir     string // ~/.argus/
	agentFile   string // ~/.argus/agent.md
	runbookDir  string // ~/.argus/runbooks/
	episodeFile string // ~/.argus/memory/episodes.json
}

// NewStore creates a Store and ensures all directories exist
func NewStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("memory store: %w", err)
	}

	baseDir    := filepath.Join(home, ".argus")
	runbookDir := filepath.Join(baseDir, "runbooks")
	memoryDir  := filepath.Join(baseDir, "memory")

	// create all dirs — MkdirAll is idempotent, safe to call every time
	for _, dir := range []string{baseDir, runbookDir, memoryDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("memory store mkdir: %w", err)
		}
	}

	s := &Store{
		baseDir:     baseDir,
		agentFile:   filepath.Join(baseDir, "agent.md"),
		runbookDir:  runbookDir,
		episodeFile: filepath.Join(memoryDir, "episodes.json"),
	}

	// create agent.md with placeholder if it doesn't exist
	if err := s.ensureAgentFile(); err != nil {
		return nil, err
	}

	return s, nil
}

// ── agent.md ──────────────────────────────────────────────────────────────

// ReadAgent returns the full contents of agent.md
// empty string if file doesn't exist yet
func (s *Store) ReadAgent() (string, error) {
	data, err := os.ReadFile(s.agentFile)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read agent.md: %w", err)
	}
	return string(data), nil
}

// AppendAgent adds a new fact to agent.md under a given section
// creates the section if it doesn't exist
// called by "remember that..." command
func (s *Store) AppendAgent(section, fact string) error {
	current, err := s.ReadAgent()
	if err != nil {
		return err
	}

	// check if section already exists
	sectionHeader := "## " + section
	if strings.Contains(current, sectionHeader) {
		// append fact under existing section
		// find the section and add after it
		updated := insertUnderSection(current, sectionHeader, "- "+fact)
		return os.WriteFile(s.agentFile, []byte(updated), 0644)
	}

	// section doesn't exist — append new section at end
	addition := fmt.Sprintf("\n## %s\n- %s\n", section, fact)
	updated := strings.TrimRight(current, "\n") + "\n" + addition
	return os.WriteFile(s.agentFile, []byte(updated), 0644)
}

// WriteAgent overwrites agent.md entirely — used by wizard
func (s *Store) WriteAgent(content string) error {
	return os.WriteFile(s.agentFile, []byte(content), 0644)
}

// ensureAgentFile creates agent.md with placeholder if missing
func (s *Store) ensureAgentFile() error {
	if _, err := os.Stat(s.agentFile); err == nil {
		return nil // already exists
	}

	placeholder := `# Argus Agent Context
> This file is read by Argus on every startup.
> Edit it directly or use "remember that..." in the query bar.

## My Stack
<!-- Add your languages, frameworks, databases here -->

## Known Services
<!-- List your services, ports, and what they do -->

## Preferences
<!-- How you like Argus to respond and suggest fixes -->

## Team Context
<!-- Solo dev, team size, cloud provider, environment details -->
`
	return os.WriteFile(s.agentFile, []byte(placeholder), 0644)
}

// ── runbooks ──────────────────────────────────────────────────────────────

// ReadRunbook returns the content of a specific runbook file
// name = "nginx", "docker", "general" etc — no extension needed
func (s *Store) ReadRunbook(name string) (string, error) {
	path := filepath.Join(s.runbookDir, name+".md")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil // no runbook for this topic yet
	}
	if err != nil {
		return "", fmt.Errorf("read runbook %s: %w", name, err)
	}
	return string(data), nil
}

// ListRunbooks returns names of all runbook files
func (s *Store) ListRunbooks() ([]string, error) {
	entries, err := os.ReadDir(s.runbookDir)
	if err != nil {
		return nil, fmt.Errorf("list runbooks: %w", err)
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			names = append(names, strings.TrimSuffix(e.Name(), ".md"))
		}
	}
	return names, nil
}

// AppendRunbook adds a step to a runbook, creating it if needed
func (s *Store) AppendRunbook(name, step string) error {
	existing, err := s.ReadRunbook(name)
	if err != nil {
		return err
	}

	var content string
	if existing == "" {
		// new runbook
		content = fmt.Sprintf("# %s Runbook\n\n## Steps\n- %s\n", name, step)
	} else {
		content = strings.TrimRight(existing, "\n") +
			fmt.Sprintf("\n- %s\n", step)
	}

	path := filepath.Join(s.runbookDir, name+".md")
	return os.WriteFile(path, []byte(content), 0644)
}

// ── helpers ───────────────────────────────────────────────────────────────

// insertUnderSection inserts a line after all the points in a given section of a markdown file.
// If the section doesn't exist, it appends the line at the end of the file.
// It assumes sections are denoted by "## Section Name" headers.
func insertUnderSection(content, header, line string) string {
	lines := strings.Split(content, "\n")
	var result []string
	
	headerIdx := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == header {
			headerIdx = i
			break
		}
	}

	// Fallback if header wasn't found natively
	if headerIdx == -1 {
		return content + "\n" + line
	}

	// Find where this section ends (either next "## " header or EOF)
	insertIdx := len(lines)
	for i := headerIdx + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "##") {
			insertIdx = i
			break
		}
	}

	// Reconstruct the file with the item appended at the section bottom
	result = append(result, lines[:insertIdx]...)
	result = append(result, line)
	result = append(result, lines[insertIdx:]...)

	return strings.Join(result, "\n")
}