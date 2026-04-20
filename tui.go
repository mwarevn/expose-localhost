package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const wrapBreakpoints = " -,.;:/"

type viewMode int

const (
	viewURL viewMode = iota
	viewLog
)

type logLineMsg string
type logDoneMsg struct{}
type copyClearMsg struct{}

type tuiModel struct {
	mode       viewMode
	url        string
	host       string
	port       string
	logs       []string
	wrapped    []string
	wrappedFor int
	maxLogs    int
	logCh      <-chan string
	width      int
	height     int
	copied     string
	copiedAt   time.Time
	logVP      viewport.Model
	vpReady    bool
}

func wrapLine(s string, w int) string {
	if w <= 0 {
		return s
	}
	return ansi.Wrap(s, w, wrapBreakpoints)
}

func (m *tuiModel) rewrapAll() {
	if !m.vpReady || m.logVP.Width <= 0 {
		m.wrapped = nil
		m.wrappedFor = 0
		return
	}
	m.wrapped = make([]string, len(m.logs))
	for i, line := range m.logs {
		m.wrapped[i] = wrapLine(line, m.logVP.Width)
	}
	m.wrappedFor = m.logVP.Width
}

func (m *tuiModel) appendLog(line string) {
	m.logs = append(m.logs, line)
	if m.vpReady && m.wrappedFor == m.logVP.Width && m.logVP.Width > 0 {
		m.wrapped = append(m.wrapped, wrapLine(line, m.logVP.Width))
	}
	if len(m.logs) > m.maxLogs {
		drop := len(m.logs) - m.maxLogs
		m.logs = m.logs[drop:]
		if len(m.wrapped) >= drop {
			m.wrapped = m.wrapped[drop:]
		} else {
			m.wrapped = nil
		}
	}
}

func readLogCmd(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return logDoneMsg{}
		}
		return logLineMsg(line)
	}
}

func clearCopyCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return copyClearMsg{} })
}

func (m tuiModel) Init() tea.Cmd { return readLogCmd(m.logCh) }

func (m *tuiModel) refreshLogContent(followBottom bool) {
	var content string
	if len(m.logs) == 0 {
		content = dim.Render("  (waiting for cloudflared output...)")
	} else if len(m.wrapped) == len(m.logs) && m.wrappedFor == m.logVP.Width {
		content = strings.Join(m.wrapped, "\n")
	} else {
		content = strings.Join(m.logs, "\n")
	}
	wasAtBottom := m.logVP.AtBottom()
	prev := m.logVP.YOffset
	m.logVP.SetContent(content)
	switch {
	case followBottom && wasAtBottom:
		m.logVP.GotoBottom()
	case followBottom:
		m.logVP.SetYOffset(prev)
	}
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		vpH := msg.Height - 8
		if vpH < 3 {
			vpH = 3
		}
		if !m.vpReady {
			m.logVP = viewport.New(msg.Width, vpH)
			m.vpReady = true
			m.rewrapAll()
			m.refreshLogContent(false)
			m.logVP.GotoBottom()
		} else if m.logVP.Width != msg.Width || m.logVP.Height != vpH {
			m.logVP.Width = msg.Width
			m.logVP.Height = vpH
			m.rewrapAll()
			m.refreshLogContent(true)
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "tab", "l":
			if m.mode == viewURL {
				m.mode = viewLog
			} else {
				m.mode = viewURL
			}
			return m, nil
		case "c", "y":
			if m.url != "" {
				if err := clipboard.WriteAll(m.url); err != nil {
					m.copied = "err"
				} else {
					m.copied = "ok"
				}
				m.copiedAt = time.Now()
				return m, clearCopyCmd(2 * time.Second)
			}
			return m, nil
		case "g":
			if m.mode == viewLog && m.vpReady {
				m.logVP.GotoTop()
			}
			return m, nil
		case "G":
			if m.mode == viewLog && m.vpReady {
				m.logVP.GotoBottom()
			}
			return m, nil
		}
		if m.mode == viewLog && m.vpReady {
			var cmd tea.Cmd
			m.logVP, cmd = m.logVP.Update(msg)
			return m, cmd
		}

	case copyClearMsg:
		m.copied = ""

	case logLineMsg:
		m.appendLog(string(msg))
		if m.vpReady {
			m.refreshLogContent(true)
		}
		return m, readLogCmd(m.logCh)

	case logDoneMsg:
		// cloudflared pipes closed; freeze on last state
	}
	return m, nil
}

func (m tuiModel) footer() string {
	k := func(key, desc string) string {
		return keyStyl.Render(key) + dim.Render(" "+desc)
	}
	sep := dim.Render("   ·   ")
	if m.mode == viewLog {
		return "  " + k("tab", "url") + sep + k("↑↓", "scroll") +
			sep + k("pgup/pgdn", "page") + sep + k("g/G", "top/bot") +
			sep + k("c", "copy url") + sep + k("q", "quit")
	}
	return "  " + k("tab", "log") + sep + k("c", "copy url") + sep + k("q", "quit")
}

func (m tuiModel) urlView() string {
	urlLine := urlStyl.Render(m.url)
	switch m.copied {
	case "ok":
		urlLine += "  " + ok.Render("✓ copied")
	case "err":
		urlLine += "  " + errStyl.Render("✗ clipboard unavailable")
	}
	content := ok.Render("✓ tunnel ready") + "\n\n" +
		dim.Render("public   ") + urlLine + "\n" +
		dim.Render("forward  ") + fmt.Sprintf("%s:%s", m.host, m.port)
	return "\n" + box.Render(content) + "\n\n" + m.footer() + "\n"
}

func (m tuiModel) logViewRender() string {
	if !m.vpReady {
		return ""
	}
	badge := lipgloss.NewStyle().Bold(true).
		Foreground(bgDark).Background(accent).
		Padding(0, 1).Render("LOG")

	var status string
	total := len(m.logs)
	if total == 0 {
		status = dim.Render("waiting...")
	} else if m.logVP.AtBottom() {
		status = ok.Render("● live") + "  " + dim.Render(fmt.Sprintf("%d lines", total))
	} else {
		pct := m.logVP.ScrollPercent() * 100
		status = dim.Render(fmt.Sprintf("paused  %.0f%%  (%d lines)", pct, total))
	}
	subtitle := dim.Render(fmt.Sprintf("%s → %s:%s", m.url, m.host, m.port))
	header := "  " + badge + "  " + subtitle + "   " + status

	return "\n" + header + "\n\n" + m.logVP.View() + "\n\n" + m.footer() + "\n"
}

func (m tuiModel) View() string {
	switch m.mode {
	case viewURL:
		return m.urlView()
	case viewLog:
		return m.logViewRender()
	}
	return ""
}

func runTUI(ctx context.Context, t *tunnel, host, port string, startInLog bool) error {
	m := tuiModel{
		url:     t.url,
		host:    host,
		port:    port,
		logCh:   t.Logs(),
		maxLogs: 10000,
	}
	if startInLog {
		m.mode = viewLog
	}
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithContext(ctx))
	_, err := p.Run()
	return err
}
