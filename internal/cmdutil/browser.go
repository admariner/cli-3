package cmdutil

import (
	"os"
	"strings"

	exec "golang.org/x/sys/execabs"
)

const ApplicationURL = "https://app.planetscale.com"

func browserCommand(goos, url string) *exec.Cmd {
	exe := "open"
	var args []string
	switch goos {
	case "darwin":
		args = append(args, url)
	case "windows":
		exe, _ = exec.LookPath("cmd")
		r := strings.NewReplacer("&", "^&")
		args = append(args, "/c", "start", r.Replace(url))
	default:
		exe = linuxExe()
		args = append(args, url)
	}

	cmd := exec.Command(exe, args...)
	cmd.Stderr = os.Stderr
	return cmd
}

// TryOpenBrowser opens the default web browser at url. It does not require a TTY.
// Callers should print the URL when this returns an error (headless CI, SSH, etc.).
func TryOpenBrowser(goos, url string) error {
	return browserCommand(goos, url).Run()
}

func linuxExe() string {
	exe := "xdg-open"

	_, err := exec.LookPath(exe)
	if err != nil {
		_, err := exec.LookPath("wslview")
		if err == nil {
			exe = "wslview"
		}
	}

	return exe
}
