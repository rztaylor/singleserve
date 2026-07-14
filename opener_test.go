package singleserve

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestBrowserCommandsByPlatform(t *testing.T) {
	tests := []struct {
		name   string
		opener systemBrowserOpener
		want   []browserCommand
	}{
		{
			name:   "macOS",
			opener: systemBrowserOpener{goos: "darwin"},
			want:   []browserCommand{{name: "open", args: []string{"http://localhost/"}}},
		},
		{
			name:   "Windows",
			opener: systemBrowserOpener{goos: "windows"},
			want:   []browserCommand{{name: "rundll32", args: []string{"url.dll,FileProtocolHandler", "http://localhost/"}}},
		},
		{
			name: "WSL",
			opener: systemBrowserOpener{
				goos:   "linux",
				getenv: func(key string) string { return map[string]string{"WSL_DISTRO_NAME": "Ubuntu"}[key] },
			},
			want: []browserCommand{
				{name: "wslview", args: []string{"http://localhost/"}},
				{name: "explorer.exe", args: []string{"http://localhost/"}},
				{name: "cmd.exe", args: []string{"/C", "start", "", "http://localhost/"}},
				{name: "xdg-open", args: []string{"http://localhost/"}},
				{name: "gio", args: []string{"open", "http://localhost/"}},
				{name: "sensible-browser", args: []string{"http://localhost/"}},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.opener.commands("http://localhost/"); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("commands = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestSystemBrowserOpenerFallsBackWithoutShell(t *testing.T) {
	var startedName string
	var startedArgs []string
	opener := systemBrowserOpener{
		goos: "linux",
		lookPath: func(name string) (string, error) {
			if name == "gio" {
				return "/usr/bin/gio", nil
			}
			return "", errors.New("missing")
		},
		start: func(_ context.Context, name string, args ...string) error {
			startedName = name
			startedArgs = append([]string(nil), args...)
			return nil
		},
	}
	if err := opener.Open(context.Background(), "http://127.0.0.1:8080/?secret=yes"); err != nil {
		t.Fatal(err)
	}
	if startedName != "gio" || !reflect.DeepEqual(startedArgs, []string{"open", "http://127.0.0.1:8080/?secret=yes"}) {
		t.Fatalf("started %q %#v", startedName, startedArgs)
	}
}

func TestLaunchOpenBrowserIsOneShot(t *testing.T) {
	var opened string
	server, err := New(Options{
		Handler: http.NotFoundHandler(),
		Opener: BrowserOpenerFunc(func(_ context.Context, rawURL string) error {
			opened = rawURL
			return nil
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	launch, err := server.Start(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = launch.Shutdown(context.Background()) })
	if err := launch.OpenBrowser(context.Background()); err != nil {
		t.Fatal(err)
	}
	if opened != launch.URL() || !strings.Contains(opened, "singleserve_token=") {
		t.Fatalf("opened URL = %q", opened)
	}
	if err := launch.OpenBrowser(context.Background()); !errors.Is(err, ErrBrowserAlreadyOpened) {
		t.Fatalf("second OpenBrowser error = %v", err)
	}
}

func TestNilBrowserOpenerFuncReturnsError(t *testing.T) {
	var opener BrowserOpener = BrowserOpenerFunc(nil)
	if err := opener.Open(context.Background(), "http://localhost/"); err == nil {
		t.Fatal("nil BrowserOpenerFunc returned no error")
	}
}
