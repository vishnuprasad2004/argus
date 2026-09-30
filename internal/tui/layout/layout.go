package layout

// Mode decides which layout the workspace uses
type Mode int

const (
    ModeWide   Mode = iota // split panes side by side
    ModeNarrow             // single pane with tab bar
)

// NarrowBreakpoint is the terminal width below which we switch to narrow mode
const NarrowBreakpoint = 120

// Dims holds all computed dimensions for one terminal size
// Every workspace screen gets one of these and sizes its components from it
type Dims struct {
    Mode Mode

    // full terminal size
    TotalWidth  int
    TotalHeight int

    // fixed region heights
    HeaderHeight  int // always 2
    TabBarHeight  int // 1 in narrow mode, 0 in wide
    FooterHeight  int // always 2
    QueryHeight   int // always 3

    // pane dimensions (wide mode)
    LogPaneWidth      int
    LogPaneHeight     int
    InsightsPaneWidth int
    InsightsPaneHeight int
    DividerCol        int // x position of the vertical divider

    // single pane dimensions (narrow mode)
    PaneWidth  int
    PaneHeight int
}

// Compute takes terminal dimensions and returns layout dims
// Call this every time WindowSizeMsg arrives
func Compute(width, height int) Dims {
    d := Dims{
        TotalWidth:   width,
        TotalHeight:  height,
        HeaderHeight: 2,
        FooterHeight: 2,
        QueryHeight:  3,
    }

    // vertical space left after fixed regions
    if width >= NarrowBreakpoint {
        d.Mode = ModeWide
        d.TabBarHeight = 0

        paneHeight := height - d.HeaderHeight - d.FooterHeight - d.QueryHeight
        if paneHeight < 1 {
            paneHeight = 1
        }

        // 65/35 split with a 1-char divider
        d.LogPaneWidth = int(float64(width) * 0.65)
        d.DividerCol   = d.LogPaneWidth
        d.InsightsPaneWidth = width - d.LogPaneWidth - 1 // 1 = divider char

        d.LogPaneHeight      = paneHeight
        d.InsightsPaneHeight = paneHeight

    } else {
        d.Mode        = ModeNarrow
        d.TabBarHeight = 1

        paneHeight := height - d.HeaderHeight - d.TabBarHeight - d.FooterHeight - d.QueryHeight
        if paneHeight < 1 {
            paneHeight = 1
        }

        d.PaneWidth  = width
        d.PaneHeight = paneHeight
    }

    return d
}

// IsWide returns true when split layout is active
func (d Dims) IsWide() bool { return d.Mode == ModeWide }