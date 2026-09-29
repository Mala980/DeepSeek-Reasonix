package openwith

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"
)

var errNoHandler = errors.New("executable file not found in $PATH")

// termuxAPI answers the way Termux's own bin directory does: every name it is
// asked for is there.
func termuxAPI(name string) (string, error) {
	return "/data/data/com.termux/files/usr/bin/" + name, nil
}

func noHandler(string) (string, error) { return "", errNoHandler }

func TestHandlerNamesTheHostHandler(t *testing.T) {
	for _, tc := range []struct {
		name     string
		folder   bool
		goos     string
		lookPath func(string) (string, error)
		want     string
		wantArgs []string
	}{
		{name: "darwin opens a url itself", goos: "darwin", lookPath: noHandler, want: "open", wantArgs: []string{"https://x"}},
		{name: "darwin opens a folder itself", folder: true, goos: "darwin", lookPath: noHandler, want: "open", wantArgs: []string{"/themes"}},
		{name: "windows url", goos: "windows", lookPath: noHandler, want: "rundll32", wantArgs: []string{"url.dll,FileProtocolHandler", "https://x"}},
		{name: "linux url", goos: "linux", lookPath: noHandler, want: "xdg-open", wantArgs: []string{"https://x"}},
		{name: "linux folder", folder: true, goos: "linux", lookPath: noHandler, want: "xdg-open", wantArgs: []string{"/themes"}},
		{name: "android url", goos: "android", lookPath: termuxAPI, want: "/data/data/com.termux/files/usr/bin/termux-open-url", wantArgs: []string{"https://x"}},
		{name: "android folder", folder: true, goos: "android", lookPath: termuxAPI, want: "/data/data/com.termux/files/usr/bin/termux-open", wantArgs: []string{"/themes"}},
		{name: "android without termux-api", goos: "android", lookPath: noHandler, want: "xdg-open", wantArgs: []string{"https://x"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := tc.wantArgs[len(tc.wantArgs)-1]
			got, gotArgs := handler(tc.folder, target, tc.goos, tc.lookPath)
			if got != tc.want {
				t.Fatalf("handler = %q, want %q", got, tc.want)
			}
			if !slices.Equal(gotArgs, tc.wantArgs) {
				t.Fatalf("args = %v, want %v", gotArgs, tc.wantArgs)
			}
		})
	}
}

// Which termux-api command a target gets is the whole Android judgement: the
// browser one for a URL, the general one for a path.
func TestAndroidAsksForTheHandlerItsTargetNeeds(t *testing.T) {
	var asked []string
	lookPath := func(name string) (string, error) {
		asked = append(asked, name)
		return "", errNoHandler
	}
	handler(false, "https://x", "android", lookPath)
	handler(true, "/themes", "android", lookPath)
	if want := []string{"termux-open-url", "termux-open"}; !slices.Equal(asked, want) {
		t.Fatalf("asked for %v, want %v", asked, want)
	}
}

func TestWindowsExplorerIsNamedByItsOwnDirectory(t *testing.T) {
	t.Setenv("SystemRoot", `C:\Windows`)
	t.Setenv("windir", "")
	if got, want := windowsExplorer(), filepath.Join(`C:\Windows`, "explorer.exe"); got != want {
		t.Fatalf("windowsExplorer = %q, want %q", got, want)
	}

	t.Setenv("SystemRoot", "")
	t.Setenv("windir", `D:\Win`)
	if got, want := windowsExplorer(), filepath.Join(`D:\Win`, "explorer.exe"); got != want {
		t.Fatalf("windowsExplorer = %q, want %q", got, want)
	}

	t.Setenv("SystemRoot", "")
	t.Setenv("windir", "")
	if got := windowsExplorer(); got != "explorer.exe" {
		t.Fatalf("windowsExplorer = %q, want the bare name for PATH to resolve", got)
	}
}

// The handler's own argument shape differs per host — rundll32 takes the URL
// after its verb — so what every platform must hold is that the target reaches
// the command line at all, and reaches it last.
func TestCommandCarriesTheTargetToTheHandler(t *testing.T) {
	cmd := URL("https://x")
	if len(cmd.Args) < 2 {
		t.Fatalf("args = %v, want a handler and its target", cmd.Args)
	}
	if last := cmd.Args[len(cmd.Args)-1]; last != "https://x" {
		t.Fatalf("args = %v, want the target last", cmd.Args)
	}
}
