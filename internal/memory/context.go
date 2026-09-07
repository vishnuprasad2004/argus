package memory

import "strings"

type Context struct {
	AgentProfile string
	Runbooks     []string
	HasMemory    bool
}

func (s *Store) Build(query string, sourceNames []string) (*Context, error) {
	ctx := &Context{}

	// first read the agent.md file
	agent, err := s.ReadAgent()
	if err != nil {
		return nil, err
	}

	// check if the agent.md file is empty
	if agent != "" && !isPlaceholderOnly(agent) {
		ctx.AgentProfile = agent
		ctx.HasMemory = true
	}

	// find relevant runbooks based on query + source names
	relevant := s.findRelevantRunbooks(query, sourceNames)
	for _, name := range relevant {
		content, err := s.ReadRunbook(name)
		if err != nil || content == "" {
			continue
		}
		ctx.Runbooks = append(ctx.Runbooks, content)
	}

	return ctx, nil
}

// findRelevantRunbooks matches query and source names to runbook files
// simple keyword matching — no embeddings needed at this stage
func (s *Store) findRelevantRunbooks(query string, sourceNames []string) []string {
	runbooks, err := s.ListRunbooks()
	if err != nil || len(runbooks) == 0 {
		return nil
	}

	queryLower := strings.ToLower(query)
	var relevant []string
	seen := map[string]bool{}

	for _, name := range runbooks {
		nameLower := strings.ToLower(name)

		// check if runbook name appears in query or source names
		if strings.Contains(queryLower, nameLower) {
			if !seen[name] {
				relevant = append(relevant, name)
				seen[name] = true
			}
			continue
		}

		for _, src := range sourceNames {
			if strings.Contains(strings.ToLower(src), nameLower) {
				if !seen[name] {
					relevant = append(relevant, name)
					seen[name] = true
				}
			}
		}
	}

	// always include "general" runbook if it exists
	for _, name := range runbooks {
		if name == "general" && !seen["general"] {
			relevant = append(relevant, "general")
		}
	}

	return relevant
}










// ToPromptString formats memory context for injection into system prompt
// returns empty string if no useful memory exists yet
func (c *Context) ToPromptString() string {
	if !c.HasMemory && len(c.Runbooks) == 0 {
		return ""
	}

	var b strings.Builder

	b.WriteString("\n\n---\n")
	b.WriteString("## Your Memory\n\n")

	if c.HasMemory {
		b.WriteString("### User Context (from agent.md)\n")
		b.WriteString(c.AgentProfile)
		b.WriteString("\n")
	}

	for _, rb := range c.Runbooks {
		b.WriteString("### Relevant Runbook\n")
		b.WriteString(rb)
		b.WriteString("\n")
	}

	b.WriteString("---\n")
	b.WriteString("Use the above context to give more relevant, personalised answers.\n")

	return b.String()
}

// isPlaceholderOnly returns true if agent.md only has placeholder comments
func isPlaceholderOnly(content string) bool {
	lines := strings.Split(content, "\n")
	realLines := 0
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" ||
			strings.HasPrefix(trimmed, "<!--") ||
			strings.HasPrefix(trimmed, ">") ||
			strings.HasPrefix(trimmed, "#") {
			continue
		}
		realLines++
	}
	return realLines == 0
}