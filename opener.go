package singleserve

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// ErrBrowserAlreadyOpened reports an attempt to open one Launch twice.
var ErrBrowserAlreadyOpened = errors.New("singleserve: browser already opened")

// BrowserOpener opens a URL without waiting for the browser to exit.
type BrowserOpener interface {
	Open(context.Context, string) error
}

// BrowserOpenerFunc adapts a function to BrowserOpener.
type BrowserOpenerFunc func(context.Context, string) error

// Open calls f.
func (f BrowserOpenerFunc) Open(ctx context.Context, rawURL string) error {
	if f == nil {
		return fmt.Errorf("singleserve: nil browser opener function")
	}
	return f(ctx, rawURL)
}

type browserCommand struct {
	name string
	args []string
}

type systemBrowserOpener struct {
	goos     string
	getenv   func(string) string
	readFile func(string) ([]byte, error)
	lookPath func(string) (string, error)
	start    func(context.Context, string, ...string) error
}

func (o systemBrowserOpener) Open(ctx context.Context, rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return fmt.Errorf("singleserve: browser URL is empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	var attempted []string
	for _, command := range o.commands(rawURL) {
		if _, err := o.pathLookup()(command.name); err != nil {
			attempted = append(attempted, command.name)
			continue
		}
		if err := o.commandStart()(ctx, command.name, command.args...); err != nil {
			attempted = append(attempted, command.name)
			continue
		}
		return nil
	}
	if len(attempted) == 0 {
		return fmt.Errorf("singleserve: no browser launch commands configured")
	}
	return fmt.Errorf("singleserve: no browser launch command worked; tried %s", strings.Join(attempted, ", "))
}

func (o systemBrowserOpener) commands(rawURL string) []browserCommand {
	switch o.platform() {
	case "darwin":
		return []browserCommand{{name: "open", args: []string{rawURL}}}
	case "windows":
		return []browserCommand{{name: "rundll32", args: []string{"url.dll,FileProtocolHandler", rawURL}}}
	case "linux":
		var commands []browserCommand
		if o.isWSL() {
			commands = append(commands,
				browserCommand{name: "wslview", args: []string{rawURL}},
				browserCommand{name: "explorer.exe", args: []string{rawURL}},
			)
		}
		return append(commands,
			browserCommand{name: "xdg-open", args: []string{rawURL}},
			browserCommand{name: "gio", args: []string{"open", rawURL}},
			browserCommand{name: "sensible-browser", args: []string{rawURL}},
		)
	default:
		return []browserCommand{
			{name: "xdg-open", args: []string{rawURL}},
			{name: "open", args: []string{rawURL}},
		}
	}
}

func (o systemBrowserOpener) isWSL() bool {
	getenv := o.getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	if strings.TrimSpace(getenv("WSL_DISTRO_NAME")) != "" || strings.TrimSpace(getenv("WSL_INTEROP")) != "" {
		return true
	}
	readFile := o.readFile
	if readFile == nil {
		readFile = os.ReadFile
	}
	data, err := readFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return false
	}
	text := strings.ToLower(string(data))
	return strings.Contains(text, "microsoft") || strings.Contains(text, "wsl")
}

func (o systemBrowserOpener) platform() string {
	if o.goos != "" {
		return o.goos
	}
	return runtime.GOOS
}

func (o systemBrowserOpener) pathLookup() func(string) (string, error) {
	if o.lookPath != nil {
		return o.lookPath
	}
	return exec.LookPath
}

func (o systemBrowserOpener) commandStart() func(context.Context, string, ...string) error {
	if o.start != nil {
		return o.start
	}
	return startBrowserCommand
}

func startBrowserCommand(ctx context.Context, name string, args ...string) error {
	command := exec.CommandContext(ctx, name, args...)
	if err := command.Start(); err != nil {
		return err
	}
	go func() {
		_ = command.Wait()
	}()
	return nil
}
