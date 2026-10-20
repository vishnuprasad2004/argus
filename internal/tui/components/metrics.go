package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/vishnuprasad2004/argus/internal/collectors/docker"
	"github.com/vishnuprasad2004/argus/internal/tui/styles"
)

// MetricsPanel renders container metrics as a compact dashboard
type MetricsPanel struct {
	metrics docker.ContainerMetrics
	width   int
	ready   bool
}

func NewMetricsPanel(width int) MetricsPanel {
	return MetricsPanel{width: width}
}

func (p *MetricsPanel) Update(m docker.ContainerMetrics) {
	p.metrics = m
	p.ready = true
}

func (p *MetricsPanel) Resize(width int) { p.width = width }

func (p MetricsPanel) View() string {
	if !p.ready {
		return "  " + styles.Muted.Render("Collecting metrics...")
	}
	if p.metrics.Err != nil {
		return "  " + styles.LogError.Render("metrics unavailable")
	}

	var b strings.Builder

	b.WriteString(styles.Muted.Render("  METRICS") + "\n")
	b.WriteString(styles.HRuleStr(p.width) + "\n")

	// CPU row
	b.WriteString(fmt.Sprintf("  %s  %s\n",
		styles.Muted.Render("CPU"),
		p.renderBar(p.metrics.CPUPercent, 100, 20)+
			styles.Base.Render(fmt.Sprintf("  %.1f%%", p.metrics.CPUPercent)),
	))

	// Memory row
	b.WriteString(fmt.Sprintf("  %s  %s\n",
		styles.Muted.Render("MEM"),
		p.renderBar(p.metrics.MemPercent, 100, 20)+
			styles.Base.Render(fmt.Sprintf("  %.0fMB / %.0fMB  %.1f%%",
				p.metrics.MemUsageMB, p.metrics.MemLimitMB, p.metrics.MemPercent)),
	))

	b.WriteString("\n")

	// Network row
	b.WriteString(fmt.Sprintf("  %s  %s  %s  %s\n",
		styles.Muted.Render("NET ↑"),
		styles.Base.Render(fmt.Sprintf("%.2f MB", p.metrics.NetOutMB)),
		styles.Muted.Render("↓"),
		styles.Base.Render(fmt.Sprintf("%.2f MB", p.metrics.NetInMB)),
	))

	// Block I/O row
	b.WriteString(fmt.Sprintf("  %s  %s  %s  %s\n",
		styles.Muted.Render("I/O R"),
		styles.Base.Render(fmt.Sprintf("%.2f MB", p.metrics.BlockReadMB)),
		styles.Muted.Render("W"),
		styles.Base.Render(fmt.Sprintf("%.2f MB", p.metrics.BlockWriteMB)),
	))

	// PIDs row
	b.WriteString(fmt.Sprintf("  %s  %s\n",
		styles.Muted.Render("PIDs"),
		styles.Base.Render(fmt.Sprintf("%d", p.metrics.PIDs)),
	))

	return b.String()
}

// renderBar draws a colored progress bar like: ████████░░░░  42%
func (p MetricsPanel) renderBar(value, max float64, width int) string {
	if max <= 0 {
		max = 100
	}
	pct := value / max
	if pct > 1 {
		pct = 1
	}

	filled := int(pct * float64(width))
	empty := width - filled

	// color based on threshold
	barColor := styles.ColorGreen
	switch {
	case pct > 0.9:
		barColor = styles.ColorRed
	case pct > 0.7:
		barColor = styles.ColorYellow
	}

	bar := lipgloss.NewStyle().Foreground(barColor).Render(strings.Repeat("█", filled)) +
		styles.Subtle.Render(strings.Repeat("░", empty))

	return bar
}
