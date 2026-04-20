package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"time"
)

const releaseBase = "https://github.com/cloudflare/cloudflared/releases/latest/download/"

type assetInfo struct {
	name  string
	isTgz bool
}

func assetFor() (assetInfo, error) {
	key := runtime.GOOS + "/" + runtime.GOARCH
	table := map[string]assetInfo{
		"linux/amd64":   {"cloudflared-linux-amd64", false},
		"linux/386":     {"cloudflared-linux-386", false},
		"linux/arm64":   {"cloudflared-linux-arm64", false},
		"linux/arm":     {"cloudflared-linux-arm", false},
		"darwin/amd64":  {"cloudflared-darwin-amd64.tgz", true},
		"darwin/arm64":  {"cloudflared-darwin-arm64.tgz", true},
		"windows/amd64": {"cloudflared-windows-amd64.exe", false},
		"windows/386":   {"cloudflared-windows-386.exe", false},
	}
	a, ok := table[key]
	if !ok {
		return assetInfo{}, fmt.Errorf("unsupported platform: %s", key)
	}
	return a, nil
}

func binPath() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cache, "expose-localhost")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := "cloudflared"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(dir, name), nil
}

func ensureCloudflared(ctx context.Context) (string, error) {
	target, err := binPath()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(target); err == nil {
		return target, nil
	}

	asset, err := assetFor()
	if err != nil {
		return "", err
	}
	url := releaseBase + asset.name

	showFirstRunNotice(asset.name, target)

	if err := downloadWithProgress(ctx, url, asset.name, target, asset.isTgz); err != nil {
		os.Remove(target)
		return "", err
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(target, 0o755); err != nil {
			return "", err
		}
	}
	return target, nil
}

func showFirstRunNotice(assetName, cachePath string) {
	platform := runtime.GOOS + "/" + runtime.GOARCH
	content := ok.Render("● first run — preparing tunnel client") + "\n\n" +
		dim.Render("asset     ") + assetName + "\n" +
		dim.Render("platform  ") + platform + "\n" +
		dim.Render("source    ") + "github.com/cloudflare/cloudflared (latest)" + "\n" +
		dim.Render("cache     ") + cachePath
	fmt.Println()
	fmt.Println(box.Render(content))
	fmt.Println()
}

func extractTgzEntry(r io.Reader, wantBase, dest string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("%s not found in archive", wantBase)
		}
		if err != nil {
			return err
		}
		if filepath.Base(h.Name) != wantBase {
			continue
		}
		f, err := os.Create(dest)
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			os.Remove(dest)
			return err
		}
		return f.Close()
	}
}

var tryURLRe = regexp.MustCompile(`https://[a-z0-9-]+\.trycloudflare\.com`)

type tunnel struct {
	cmd   *exec.Cmd
	url   string
	logCh chan string
}

func (t *tunnel) Stop() {
	if t.cmd != nil && t.cmd.Process != nil {
		_ = t.cmd.Process.Kill()
		_ = t.cmd.Wait()
	}
}

func (t *tunnel) Logs() <-chan string { return t.logCh }

func startTunnel(ctx context.Context, bin, host, port string) (*tunnel, error) {
	cmd := exec.CommandContext(ctx, bin,
		"tunnel", "--no-autoupdate", "--url", fmt.Sprintf("%s:%s", host, port))

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	t := &tunnel{cmd: cmd, logCh: make(chan string, 512)}
	urlCh := make(chan string, 1)

	var wg sync.WaitGroup
	wg.Add(2)
	scan := func(r io.Reader) {
		defer wg.Done()
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 64*1024), 1<<20)
		for sc.Scan() {
			line := sc.Text()
			if m := tryURLRe.FindString(line); m != "" {
				select {
				case urlCh <- m:
				default:
				}
			}
			// push to ring buffer; drop oldest if full
			select {
			case t.logCh <- line:
			default:
				select {
				case <-t.logCh:
				default:
				}
				select {
				case t.logCh <- line:
				default:
				}
			}
		}
	}
	go scan(stderr)
	go scan(stdout)
	go func() { wg.Wait(); close(t.logCh) }()

	select {
	case url := <-urlCh:
		t.url = url
		return t, nil
	case <-time.After(36 * time.Second):
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, fmt.Errorf("timeout: no tunnel URL after 36s")
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, ctx.Err()
	}
}
