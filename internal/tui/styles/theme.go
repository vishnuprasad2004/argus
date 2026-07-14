package styles

import (
    "github.com/charmbracelet/lipgloss"
    "strings"
    "github.com/charmbracelet/glamour"
)

// ── Palette — Claude Code inspired ───────────────────────────────────────
var (
    ColorBg       = lipgloss.Color("#0a0a0a") // near black
    ColorSurface  = lipgloss.Color("#1a1a1a") // slightly lighter
    ColorBorder   = lipgloss.Color("#2a2a2a") // very subtle separator
    ColorPrimary  = lipgloss.Color("#cc785c") // claude orange-brown
    ColorBlue     = lipgloss.Color("#5c9fcc") // blue for info
    ColorGreen    = lipgloss.Color("#5ccc8a") // green for success
    ColorYellow   = lipgloss.Color("#ccb85c") // yellow for warn
    ColorRed      = lipgloss.Color("#cc5c5c") // red for error
    ColorMuted    = lipgloss.Color("#666666") // timestamps, hints
    ColorSubtle   = lipgloss.Color("#444444") // very faint — separators
    ColorText     = lipgloss.Color("#dddddd") // primary text
    ColorAccent   = lipgloss.Color("#cc785c") // same as primary
)

// ── Base styles ───────────────────────────────────────────────────────────
var (
    Base = lipgloss.NewStyle().Foreground(ColorText)
    Muted = lipgloss.NewStyle().Foreground(ColorMuted)
    Subtle = lipgloss.NewStyle().Foreground(ColorSubtle)
    Link = lipgloss.NewStyle().Foreground(ColorBlue).Underline(true).Italic(true)

    // used for section headers like "◆ Argus"
    Brand = lipgloss.NewStyle().
        Foreground(ColorPrimary).
        Bold(true)

    Title = lipgloss.NewStyle().
        Foreground(ColorPrimary).
        Bold(true)
)

// ── Log level styles ──────────────────────────────────────────────────────
var (
    LogError = lipgloss.NewStyle().Foreground(ColorRed)
    LogWarn  = lipgloss.NewStyle().Foreground(ColorYellow)
    LogInfo  = lipgloss.NewStyle().Foreground(ColorText)
    LogDebug = lipgloss.NewStyle().Foreground(ColorMuted)
)

// ── No heavy borders — use slim separators instead ────────────────────────
var (
    // horizontal rule — like ────────────────
    HRule = lipgloss.NewStyle().Foreground(ColorSubtle)

    // input bar — minimal, just a left accent
    InputBar = lipgloss.NewStyle().
        BorderLeft(true).
        BorderStyle(lipgloss.ThickBorder()).
        BorderForeground(ColorPrimary).
        PaddingLeft(1)

    // agent event line
    AgentEvent = lipgloss.NewStyle().Foreground(ColorMuted)
    AgentTool  = lipgloss.NewStyle().Foreground(ColorBlue)
    AgentAnswer = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
)

// ── Selector ──────────────────────────────────────────────────────────────
var (
    SelectorCursor      = "❯ "
    SelectorItem        = lipgloss.NewStyle().Foreground(ColorText).PaddingLeft(2)
    SelectorItemActive  = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
    SelectorDesc        = lipgloss.NewStyle().Foreground(ColorMuted)
)

// ── Status dots ───────────────────────────────────────────────────────────
func StatusDot(running bool) string {
    if running {
        return lipgloss.NewStyle().Foreground(ColorGreen).Render("●")
    }
    return lipgloss.NewStyle().Foreground(ColorRed).Render("●")
}

// ── Log level helper ──────────────────────────────────────────────────────
func LogStyle(level string) lipgloss.Style {
    switch level {
    case "ERROR", "FATAL":
        return LogError
    case "WARN":
        return LogWarn
    case "DEBUG":
        return LogDebug
    default:
        return LogInfo
    }
}

// ── HRuleStr returns a full-width separator line ──────────────────────────
func HRuleStr(width int) string {
    if width <= 0 { width = 80 }
    return HRule.Render(strings.Repeat("─", width))
}

// MarkdownRenderer renders LLM markdown output to styled terminal text
// created once, reused everywhere — width is set dynamically per call
func NewMarkdownRenderer(width int) (*glamour.TermRenderer, error) {
    return glamour.NewTermRenderer(
        glamour.WithStandardStyle("dark"), // dark theme matches our palette
        glamour.WithWordWrap(width),
    )
}

// RenderMarkdown converts markdown string to styled terminal output
// falls back to plain wrapped text if parsing fails
func RenderMarkdown(text string, width int) string {
    r, err := NewMarkdownRenderer(width)
    if err != nil {
        return WrapText(text, width) // fallback
    }
    out, err := r.Render(text)
    if err != nil {
        return WrapText(text, width)
    }
    return strings.TrimRight(out, "\n") // glamour adds trailing newlines
}

func WrapText(text string, width int) string {
    if width <= 0 {
        width = 80
    }
    return lipgloss.NewStyle().Width(width).Render(text)
}