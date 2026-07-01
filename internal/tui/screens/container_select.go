package screens

import (
    "context"
    "fmt"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/bubbles/spinner"
    "github.com/vishnuprasad2004/argus/internal/collectors/docker"
    "github.com/vishnuprasad2004/argus/internal/tui/styles"
)

// containerLoadedMsg is sent when Docker responds with container list
type containerLoadedMsg struct {
    targets []docker.ContainerTarget
    err     error
}

type ContainerSelectModel struct {
    source    string
    targets   []docker.ContainerTarget
    cursor    int
    loading   bool          // true while fetching from Docker
    err       error
    spinner   spinner.Model // bubbles spinner for loading state
    collector *docker.DockerCollector
}

func NewContainerSelectModel(source string) ContainerSelectModel {
    sp := spinner.New()
    sp.Spinner = spinner.Dot
    sp.Style = styles.Muted

    return ContainerSelectModel{
        source:  source,
        loading: true,
        spinner: sp,
    }
}

func (m ContainerSelectModel) Init() tea.Cmd {
    return tea.Batch(
        m.spinner.Tick,   // start spinner animation
        m.loadContainers, // fetch containers from Docker
    )
}

// loadContainers is a tea.Cmd — runs in background, sends msg when done
// this is the bubbletea pattern for async work
// like a Promise in JS — returns a function that returns a message
func (m ContainerSelectModel) loadContainers() tea.Msg {
    collector, err := docker.NewDockerCollector()
    if err != nil {
        return containerLoadedMsg{err: err}
    }

    ctx := context.Background()
    if err := collector.Validate(ctx); err != nil {
        return containerLoadedMsg{err: fmt.Errorf("docker daemon not running: %w", err)}
    }

    targets, err := collector.ListContainers(ctx)
    return containerLoadedMsg{targets: targets, err: err}
}

func (m ContainerSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {

    case containerLoadedMsg:
        m.loading = false
        m.err = msg.err
        m.targets = msg.targets
        return m, nil

    case tea.KeyMsg:
        if m.loading { return m, nil } // ignore keys while loading

        switch msg.String() {
        case "up", "k":
            if m.cursor > 0 { m.cursor-- }
        case "down", "j":
            if m.cursor < len(m.targets)-1 { m.cursor++ }
        case "enter":
            if len(m.targets) == 0 { return m, nil }
            selected := m.targets[m.cursor]
            return m, func() tea.Msg {
                return SwitchToChat{Target: selected}
            }
        case "b": // b = go back
            return m, func() tea.Msg { return SwitchToSourceSelect{} }
        }

    default:
        // forward all other messages to spinner so it animates
        var cmd tea.Cmd
        m.spinner, cmd = m.spinner.Update(msg)
        return m, cmd
    }

    return m, nil
}

func (m ContainerSelectModel) View() string {
    title := styles.Title.Render("Select Container")

    // loading state
    if m.loading {
        return title + "\n\n" + m.spinner.View() + " Connecting to Docker...\n"
    }

    // error state
    if m.err != nil {
        errMsg := styles.LogError.Render("✗ " + m.err.Error())
        hint   := styles.Muted.Render("\nMake sure Docker is running and try again.")
        return title + "\n\n" + errMsg + hint
    }

    // empty state
    if len(m.targets) == 0 {
        empty := styles.Muted.Render("No containers found. Start one and press [r] to refresh.")
        return title + "\n\n" + empty
    }

    hint := styles.Muted.Render("↑/↓ to move   enter to select   b to go back")
    out  := title + "\n" + hint + "\n\n"

    for i, t := range m.targets {
        running := t.Status == "running"
        dot     := styles.StatusDot(running)
        status  := styles.Muted.Render(t.Status)

        var row string
        if i == m.cursor {
            cursor := styles.AgentTool.Render(styles.SelectorCursor)
            name   := styles.SelectorItemActive.Render(t.Name)
            row     = fmt.Sprintf("%s%s %s  %s  %s", cursor, dot, name, t.Image, status)
        } else {
            name := styles.SelectorItem.Render(t.Name)
            row   = fmt.Sprintf("  %s %s  %s  %s", dot, name, t.Image, status)
        }

        out += row + "\n"
    }

    return out
}