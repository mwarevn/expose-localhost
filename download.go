package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
)

type progressMsg int64
type downloadDoneMsg struct{}

type downloadModel struct {
	progress  progress.Model
	total     int64
	received  int64
	assetName string
}

func (m downloadModel) Init() tea.Cmd { return nil }

func (m downloadModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		w := msg.Width - 10
		if w > 60 {
			w = 60
		}
		if w < 20 {
			w = 20
		}
		m.progress.Width = w
		return m, nil
	case tea.KeyMsg:
		if k := msg.String(); k == "ctrl+c" || k == "q" || k == "esc" {
			return m, tea.Quit
		}
	case progressMsg:
		m.received = int64(msg)
		var pct float64
		if m.total > 0 {
			pct = float64(m.received) / float64(m.total)
			if pct > 1 {
				pct = 1
			}
		}
		return m, m.progress.SetPercent(pct)
	case downloadDoneMsg:
		return m, tea.Quit
	case progress.FrameMsg:
		pm, cmd := m.progress.Update(msg)
		m.progress = pm.(progress.Model)
		return m, cmd
	}
	return m, nil
}

func (m downloadModel) View() string {
	totalStr := "?"
	if m.total > 0 {
		totalStr = humanBytes(m.total)
	}
	header := "  " + brand.Render("↓ downloading ") + m.assetName
	bar := "  " + m.progress.View()
	stats := "  " + dim.Render(fmt.Sprintf("%s / %s", humanBytes(m.received), totalStr))
	return "\n" + header + "\n\n" + bar + "\n\n" + stats + "\n"
}

type progressReader struct {
	r        io.Reader
	received int64
	onTick   func(int64)
	lastTick time.Time
}

func (p *progressReader) Read(buf []byte) (int, error) {
	n, err := p.r.Read(buf)
	if n > 0 {
		p.received += int64(n)
		if time.Since(p.lastTick) >= 40*time.Millisecond || err != nil {
			p.onTick(p.received)
			p.lastTick = time.Now()
		}
	}
	return n, err
}

func downloadWithProgress(ctx context.Context, url, assetName, dest string, isTgz bool) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	bar := progress.New(progress.WithGradient("#F38BA8", "#89B4FA"))
	bar.Width = 50
	m := downloadModel{
		progress:  bar,
		total:     resp.ContentLength,
		assetName: assetName,
	}

	p := tea.NewProgram(m)

	errCh := make(chan error, 1)
	go func() {
		pr := &progressReader{
			r:      resp.Body,
			onTick: func(n int64) { p.Send(progressMsg(n)) },
		}
		var e error
		if isTgz {
			e = extractTgzEntry(pr, "cloudflared", dest)
		} else {
			f, ferr := os.Create(dest)
			if ferr != nil {
				e = ferr
			} else {
				_, e = io.Copy(f, pr)
				if cerr := f.Close(); e == nil {
					e = cerr
				}
			}
		}
		p.Send(progressMsg(pr.received))
		time.Sleep(120 * time.Millisecond)
		p.Send(downloadDoneMsg{})
		errCh <- e
	}()

	if _, runErr := p.Run(); runErr != nil {
		return runErr
	}
	return <-errCh
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
