package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/charmbracelet/lipgloss"
)

var (
	accent  = lipgloss.Color("#F38BA8") // pink — focus / borders
	mauve   = lipgloss.Color("#CBA6F7") // purple — brand
	green   = lipgloss.Color("#A6E3A1")
	blue    = lipgloss.Color("#89B4FA")
	muted   = lipgloss.Color("#6C7086")
	fg      = lipgloss.Color("#CDD6F4")
	bgDark  = lipgloss.Color("#1E1E2E")

	brand   = lipgloss.NewStyle().Bold(true).Foreground(mauve)
	dim     = lipgloss.NewStyle().Foreground(muted)
	ok      = lipgloss.NewStyle().Bold(true).Foreground(green)
	urlStyl = lipgloss.NewStyle().Bold(true).Underline(true).Foreground(blue)
	errStyl = lipgloss.NewStyle().Bold(true).Foreground(accent)
	keyStyl = lipgloss.NewStyle().Bold(true).Foreground(accent)
	box     = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Padding(1, 2)
)

func main() {
	opts := parseFlags()

	fmt.Println()
	fmt.Println("  " + brand.Render("expose-localhost") + dim.Render("  ·  cloudflared tunnel"))
	fmt.Println()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	bin, err := ensureCloudflared(ctx)
	if err != nil {
		fail(err)
	}

	host, port := opts.host, opts.port
	initialView := "url"

	if opts.skipForm {
		if err := validateHost(host); err != nil {
			fail(fmt.Errorf("invalid host: %w", err))
		}
		if err := validatePort(port); err != nil {
			fail(fmt.Errorf("invalid port: %w", err))
		}
	} else {
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Local host").
					Description("The address cloudflared should forward traffic to.").
					Value(&host).
					Validate(validateHost),
				huh.NewInput().
					Title("Local port").
					Description("The port your app is listening on (1 – 65535).").
					Value(&port).
					Validate(validatePort),
				huh.NewSelect[string]().
					Title("Initial view").
					Description("You can press tab later to toggle url ↔ log at any time.").
					Options(
						huh.NewOption("URL   —  show the public tunnel url", "url"),
						huh.NewOption("Log   —  stream cloudflared logs",    "log"),
					).
					Value(&initialView),
			),
		).WithTheme(formTheme()).WithShowHelp(true)
		if err := form.Run(); err != nil {
			fail(err)
		}
	}

	var t *tunnel
	var startErr error
	_ = spinner.New().
		Title(" starting tunnel — waiting for cloudflared to assign a public url...").
		Action(func() { t, startErr = startTunnel(ctx, bin, host, port) }).
		Run()
	if startErr != nil {
		fail(startErr)
	}
	defer t.Stop()

	if opts.skipForm {
		runShortHand(ctx, t, host, port, opts.streamLog)
	} else {
		if err := runTUI(ctx, t, host, port, initialView == "log"); err != nil {
			fail(err)
		}
		if t.url != "" {
			fmt.Println()
			fmt.Println(dim.Render("  last url: ") + urlStyl.Render(t.url))
		}
	}

	fmt.Println()
	fmt.Println(dim.Render("  tunnel stopped"))
}

func runShortHand(ctx context.Context, t *tunnel, host, port string, streamLog bool) {
	content := ok.Render("✓ tunnel ready") + "\n\n" +
		dim.Render("public  ") + urlStyl.Render(t.url) + "\n" +
		dim.Render("forward ") + fmt.Sprintf("%s:%s", host, port)
	fmt.Println()
	fmt.Println(box.Render(content))
	fmt.Println()

	if !streamLog {
		fmt.Println(dim.Render("  press Ctrl+C to stop"))
		<-ctx.Done()
		return
	}

	fmt.Println(dim.Render("  streaming cloudflared logs (Ctrl+C to stop) ↓"))
	fmt.Println()
	for {
		select {
		case <-ctx.Done():
			return
		case line, ok := <-t.Logs():
			if !ok {
				return
			}
			fmt.Println(line)
		}
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, errStyl.Render("error: ")+err.Error())
	os.Exit(1)
}

func parseFlags() options {
	opts := options{host: "127.0.0.1", port: "8080"}

	fs := flag.CommandLine
	fs.StringVar(&opts.host, "h", opts.host, "host / IP to expose")
	fs.StringVar(&opts.host, "host", opts.host, "host / IP to expose")
	fs.StringVar(&opts.port, "p", opts.port, "local port to forward")
	fs.StringVar(&opts.port, "port", opts.port, "local port to forward")
	fs.BoolVar(&opts.streamLog, "log", false, "stream cloudflared logs to stdout (shorthand mode)")
	fs.Usage = func() {
		out := fs.Output()
		fmt.Fprintln(out, "Usage: expose-localhost [-h host] [-p port] [--log]")
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "Flags:")
		fmt.Fprintln(out, "  -h, --host string   host or IP to expose (default 127.0.0.1)")
		fmt.Fprintln(out, "  -p, --port string   local port to forward (default 8080)")
		fmt.Fprintln(out, "      --log           stream cloudflared logs after the URL is ready")
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "With no flags, you'll be prompted interactively (tab toggles url/log).")
		fmt.Fprintln(out, "With both -h and -p set, the tunnel starts immediately.")
	}
	flag.Parse()

	explicit := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	opts.skipForm = (explicit["h"] || explicit["host"]) && (explicit["p"] || explicit["port"])
	return opts
}

type options struct {
	host, port string
	skipForm   bool
	streamLog  bool
}

func validateHost(s string) error {
	if s == "" {
		return fmt.Errorf("host cannot be empty")
	}
	if ip := net.ParseIP(s); ip != nil {
		return nil
	}
	if _, err := net.LookupHost(s); err != nil {
		return fmt.Errorf("invalid host or cannot resolve")
	}
	return nil
}

func validatePort(s string) error {
	p, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("port must be a number")
	}
	if p < 1 || p > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
}

func formTheme() *huh.Theme {
	t := huh.ThemeCharm()
	t.Focused.Base = t.Focused.Base.BorderForeground(accent)
	t.Focused.Title = t.Focused.Title.Foreground(accent).Bold(true)
	t.Focused.NoteTitle = t.Focused.NoteTitle.Foreground(accent)
	t.Focused.Description = t.Focused.Description.Foreground(muted)
	t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(accent)
	t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(accent)
	t.Focused.TextInput.Text = t.Focused.TextInput.Text.Foreground(fg)
	t.Focused.TextInput.Placeholder = t.Focused.TextInput.Placeholder.Foreground(muted)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(accent).Bold(true)
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(accent)
	t.Focused.Option = t.Focused.Option.Foreground(fg)
	t.Focused.FocusedButton = t.Focused.FocusedButton.Foreground(bgDark).Background(accent).Bold(true)
	t.Focused.BlurredButton = t.Focused.BlurredButton.Foreground(muted)
	t.Focused.ErrorIndicator = t.Focused.ErrorIndicator.Foreground(accent)
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(accent)
	t.Blurred.Title = t.Blurred.Title.Foreground(muted)
	t.Blurred.Description = t.Blurred.Description.Foreground(muted)
	t.Blurred.SelectedOption = t.Blurred.SelectedOption.Foreground(muted)
	t.Help.ShortKey = t.Help.ShortKey.Foreground(accent)
	t.Help.ShortDesc = t.Help.ShortDesc.Foreground(muted)
	t.Help.FullKey = t.Help.FullKey.Foreground(accent)
	t.Help.FullDesc = t.Help.FullDesc.Foreground(muted)
	return t
}
