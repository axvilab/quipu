package ui

import (
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// runtimeFile records the URL of the UI served by a running daemon, token included, so that
// `quipu ui` can open it. Only the user can read it.
const runtimeFile = "ui-url"

// WriteRuntime stores url for other quipu processes; RemoveRuntime deletes it on exit.
func WriteRuntime(dataDir, url string) error {
	return os.WriteFile(filepath.Join(dataDir, runtimeFile), []byte(url+"\n"), 0o600)
}

// RemoveRuntime deletes the file written by WriteRuntime.
func RemoveRuntime(dataDir string) {
	os.Remove(filepath.Join(dataDir, runtimeFile))
}

// Running returns the URL of a UI served by a live daemon, if there is one.
func Running(dataDir string) (string, bool) {
	b, err := os.ReadFile(filepath.Join(dataDir, runtimeFile))
	if err != nil {
		return "", false
	}
	url := strings.TrimSpace(string(b))
	base, _, _ := strings.Cut(url, "/?")
	client := http.Client{Timeout: time.Second}
	resp, err := client.Get(base + "/healthz")
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64))
	return url, resp.StatusCode == http.StatusOK && string(body) == healthReply
}

// Friendly returns url with 127.0.0.1 replaced by quipu.localhost, the address shown to people.
func Friendly(url string) string {
	return strings.Replace(url, "://127.0.0.1:", "://"+Hostname+":", 1)
}

// Icon is the application icon (SVG), also installed for the menu entry.
func Icon() []byte {
	b, _ := assets.ReadFile("static/icon.svg")
	return b
}

func init() {
	// Browsers want the right type for the app manifest; Go's table does not know this one.
	mime.AddExtensionType(".webmanifest", "application/manifest+json")
}
