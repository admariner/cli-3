package cmdutil

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	exec "golang.org/x/sys/execabs"
)

const ApplicationURL = "https://app.planetscale.com"

// validateBrowserURL rejects URLs that are unsafe to pass to an OS opener.
// Only http(s) is allowed, and a leading '-' is rejected so open/xdg-open
// cannot treat the value as a command-line flag.
func validateBrowserURL(raw string) error {
	if raw == "" {
		return fmt.Errorf("URL must not be empty")
	}
	if strings.HasPrefix(raw, "-") {
		return fmt.Errorf("URL must not start with '-'")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	switch strings.ToLower(u.Scheme) {
	case "http", "https":
	default:
		return fmt.Errorf("unsupported URL scheme %q (want http or https)", u.Scheme)
	}

	if u.Host == "" {
		return fmt.Errorf("URL must include a host")
	}

	return nil
}

func browserCommand(goos, rawURL string) (*exec.Cmd, error) {
	if err := validateBrowserURL(rawURL); err != nil {
		return nil, err
	}

	exe := "open"
	var args []string
	switch goos {
	case "darwin":
		args = append(args, rawURL)
	case "windows":
		// Prefer rundll32 over `cmd /c start` so the URL is not re-parsed by
		// cmd.exe (which would honor metacharacters like | < > ^).
		exe = "rundll32"
		args = append(args, "url.dll,FileProtocolHandler", rawURL)
	default:
		exe = linuxExe()
		args = append(args, rawURL)
	}

	cmd := exec.Command(exe, args...)
	cmd.Stderr = os.Stderr
	return cmd, nil
}

// TryOpenBrowser opens the default web browser at url. It does not require a TTY.
// Callers should print the URL when this returns an error (headless CI, SSH, etc.).
// Only http and https URLs are opened; other schemes and values starting with '-'
// are rejected.
func TryOpenBrowser(goos, url string) error {
	cmd, err := browserCommand(goos, url)
	if err != nil {
		return err
	}
	return cmd.Run()
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
