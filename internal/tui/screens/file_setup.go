package screens

import (
    "strconv"
    "strings"

    "github.com/charmbracelet/bubbles/textinput"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/vishnuprasad2004/argus/internal/collectors/file"
    "github.com/vishnuprasad2004/argus/internal/tui/styles"
)

type SwitchToFileChat struct {
    Collector *file.FileCollector
    FullFile  bool
    TailLines int
}

// step 1 = path input, step 2 = lines choice
type fileSetupStep int
const (
    fileStepPath  fileSetupStep = iota
    fileStepLines
)

type FileSetupModel struct {
    step      fileSetupStep
    pathInput textinput.Model
    linesInput textinput.Model
    collector *file.FileCollector
    err       string
}

func NewFileSetupModel() FileSetupModel {
    // path input
    pi := textinput.New()
    pi.Placeholder = "/var/log/nginx/error.log or ~/logs/app.log"
    pi.Focus()
    pi.PromptStyle      = styles.Brand
    pi.TextStyle        = styles.Base
    pi.PlaceholderStyle = styles.Muted
    pi.Prompt           = "❯ "
    pi.CharLimit        = 300
    pi.Width            = 60

    // lines input
    li := textinput.New()
    li.Placeholder = "500"
    li.PromptStyle      = styles.Brand
    li.TextStyle        = styles.Base
    li.PlaceholderStyle = styles.Muted
    li.Prompt           = "❯ "
    li.CharLimit        = 10
    li.Width            = 20

    return FileSetupModel{
        step:       fileStepPath,
        pathInput:  pi,
        linesInput: li,
    }
}

func (m FileSetupModel) Init() tea.Cmd {
    return textinput.Blink
}

func (m FileSetupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {

        case "esc":
            return m, func() tea.Msg { return SwitchToSourceSelect{} }

        case "enter":
            switch m.step {

            // ── step 1: validate path ─────────────────────────────────
            case fileStepPath:
                path := strings.TrimSpace(m.pathInput.Value())
                if path == "" {
                    m.err = "please enter a file path"
                    return m, nil
                }

                collector, err := file.NewFileCollector(path)
                if err != nil {
                    m.err = err.Error()
                    return m, nil
                }

                // path valid — move to step 2
                m.collector = collector
                m.err = ""
                m.step = fileStepLines
                m.pathInput.Blur()
                m.linesInput.Focus()
                return m, textinput.Blink

            // ── step 2: choose lines or full file ─────────────────────
            case fileStepLines:
                val := strings.TrimSpace(m.linesInput.Value())

                // empty or "all" = full file
                if val == "" || strings.ToLower(val) == "all" {
                    return m, func() tea.Msg {
                        return SwitchToFileChat{
                            Collector: m.collector,
                            FullFile:  true,
                        }
                    }
                }

                // parse number
                n, err := strconv.Atoi(val)
                if err != nil || n <= 0 {
                    m.err = "enter a number (e.g. 500) or 'all'"
                    return m, nil
                }

                return m, func() tea.Msg {
                    return SwitchToFileChat{
                        Collector: m.collector,
                        FullFile:  false,
                        TailLines: n,
                    }
                }
            }

        // f = full file shortcut on step 2
        case "f":
            if m.step == fileStepLines {
                return m, func() tea.Msg {
                    return SwitchToFileChat{
                        Collector: m.collector,
                        FullFile:  true,
                    }
                }
            }
        }
    }

    // update active input
    var cmd tea.Cmd
    if m.step == fileStepPath {
        m.pathInput, cmd = m.pathInput.Update(msg)
    } else {
        m.linesInput, cmd = m.linesInput.Update(msg)
    }
    return m, cmd
}

func (m FileSetupModel) View() string {
    var b strings.Builder

    b.WriteString("\n")
    b.WriteString(styles.HRuleStr(60) + "\n\n")
    b.WriteString("  " + styles.Title.Render("Analyze a log file") + "\n\n")

    switch m.step {

    case fileStepPath:
        b.WriteString("  " + styles.Muted.Render("Supports .log .txt and JSON (newline-delimited)") + "\n\n")
        b.WriteString(styles.HRuleStr(60) + "\n\n")
        b.WriteString("  " + styles.Muted.Render("File path") + "\n")
        b.WriteString("  " + m.pathInput.View() + "\n\n")

    case fileStepLines:
        b.WriteString("  " + styles.Brand.Render("✓ ") +
            styles.Muted.Render(m.collector.Path()) + "\n\n")
        b.WriteString(styles.HRuleStr(60) + "\n\n")
        b.WriteString("  " + styles.Muted.Render("How many lines? (enter a number, 'all', or press f for full file)") + "\n")
        b.WriteString("  " + m.linesInput.View() + "\n\n")
        b.WriteString("  " + styles.Muted.Render("tip: large files with 'all' may use more Gemini tokens") + "\n\n")
    }

    if m.err != "" {
        b.WriteString("  " + styles.LogError.Render("✗ "+m.err) + "\n\n")
    }

    b.WriteString(styles.HRuleStr(60) + "\n")
    b.WriteString("  " + styles.Muted.Render("enter confirm   esc back") + "\n")

    return b.String()
}