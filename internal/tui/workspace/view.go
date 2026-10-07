package workspace

import (
	"strings"

	"github.com/vishnuprasad2004/argus/internal/tui/layout"
	"github.com/vishnuprasad2004/argus/internal/tui/styles"
)

// RenderView assembles the full screen from a pre-built header.
// Decision: header is passed in so each screen controls its own source info
// without Core needing to know about Docker/process/file.
func (c *Core) RenderView(header string) string {
	if c.Width == 0 || c.LogViewer == nil {
		return "\n  " + styles.Muted.Render("initializing...")
	}

	thinkingStr := c.Thinking.View()
	queryStr    := "\n" + c.QueryBar.View() + "\n"
	footer      := c.renderFooter()

	// insights content
	var insightsContent string
	if len(c.AnswerContent) == 0 {
		insightsContent = "  " + styles.Muted.Render("ask a question below...")
	} else {
		insightsContent = c.AnswerVP.View()
	}
	if thinkingStr != "" {
		insightsContent += "\n" + thinkingStr
	}

	var b strings.Builder
	b.WriteString(header)

	if c.Dims.IsWide() {
		logBorder := styles.HRuleStr(c.Dims.LogPaneWidth)
		if c.FocusedPanel == 0 {
			logBorder = styles.Brand.Render(strings.Repeat("─", c.Dims.LogPaneWidth))
		}
		insBorder := styles.HRuleStr(c.Dims.InsightsPaneWidth)
		if c.FocusedPanel == 1 {
			insBorder = styles.Brand.Render(strings.Repeat("─", c.Dims.InsightsPaneWidth))
		}

		left    := logBorder + "\n" + c.LogViewer.View()
		right   := insBorder + "\n" + insightsContent
		divider := layout.Divider(c.Dims.LogPaneHeight + 1)
		panes   := layout.JoinPanes(left, divider, right)

		b.WriteString(panes)
		b.WriteString("\n")
	} else {
		b.WriteString(layout.TabBar(c.Dims.PaneWidth, c.FocusedPanel))
		b.WriteString("\n")
		if c.FocusedPanel == 0 {
			b.WriteString(styles.Brand.Render(strings.Repeat("─", c.Width)) + "\n")
			b.WriteString(c.LogViewer.View())
		} else {
			b.WriteString(styles.Brand.Render(strings.Repeat("─", c.Width)) + "\n")
			b.WriteString(insightsContent)
		}
		b.WriteString("\n")
	}

	b.WriteString(queryStr)
	b.WriteString(footer)
	return b.String()
}

func (c *Core) renderFooter() string {
	focused := map[int]string{0: "logs", 1: "answers"}[c.FocusedPanel]

	shortcuts := []struct{ key, desc string }{
		{"↑↓",    "scroll"},
		{"f",     "follow"},
		{"tab",   "switch panel"},
		{"esc",   "back"},
		{"/stats","counts"},
		{"/clear","clear"},
		{"/quit", "quit"},
	}

	var keys, descs strings.Builder
	for i, s := range shortcuts {
		keys.WriteString(styles.Brand.Render(s.key))
		descs.WriteString(styles.Muted.Render(s.desc))
		if i < len(shortcuts)-1 {
			keys.WriteString(styles.Muted.Render("  ·  "))
			descs.WriteString(styles.Muted.Render("  ·  "))
		}
	}

	return styles.HRuleStr(c.Width) + "\n" +
		"  " + keys.String() + "\n" +
		"  " + descs.String() + "\n" +
		"  " + styles.Muted.Render("panel: "+focused) + "\n"
}