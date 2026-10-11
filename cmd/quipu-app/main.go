// Command quipu-app shows the quipu UI in a native window (ADR 0007).
//
// It is a thin shell: `quipu ui` starts it with the address of a running quipu server in
// QUIPU_UI_URL (token included; the environment, unlike the command line, is private to the
// user), and the window forwards every request to that server. The app holds no data and no
// database, so it stays small, and only one process collects transcripts.
package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// URLEnv carries the server address from `quipu ui`.
const URLEnv = "QUIPU_UI_URL"

var version = "dev"

//go:embed icon.png
var icon []byte

func main() {
	// `quipu ui` runs this first: where GTK or WebKit is missing, the dynamic loader fails before
	// main and quipu falls back to the browser.
	if len(os.Args) > 1 && os.Args[1] == "-version" {
		fmt.Println("quipu-app", version)
		return
	}
	if err := run(os.Getenv(URLEnv)); err != nil {
		fmt.Fprintln(os.Stderr, "quipu-app:", err)
		os.Exit(1)
	}
}

func run(raw string) error {
	proxy, err := forward(raw)
	if err != nil {
		return err
	}
	var appCtx context.Context
	return wails.Run(&options.App{
		Title:            "Quipu",
		Width:            1280,
		Height:           840,
		MinWidth:         900,
		MinHeight:        600,
		BackgroundColour: options.NewRGB(0x17, 0x1a, 0x23), // night, so there is no white flash
		AssetServer:      &assetserver.Options{Handler: proxy},
		OnStartup:        func(ctx context.Context) { appCtx = ctx },
		// Opening quipu again brings the open window forward instead of a second one.
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "dev.axvilab.quipu",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				runtime.WindowUnminimise(appCtx)
				runtime.Show(appCtx)
			},
		},
		Linux: &linux.Options{
			Icon:             icon,
			ProgramName:      "quipu", // the window's app id, matched to quipu.desktop for the icon
			WebviewGpuPolicy: linux.WebviewGpuPolicyOnDemand,
		},
	})
}

// forward returns a handler that sends the webview's requests to the quipu server at raw
// (http://127.0.0.1:port/?t=token), as if they came from a browser tab of that server: with the
// token cookie, the server's own Host and, for changes, its own Origin.
func forward(raw string) (http.Handler, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.Host == "" {
		return nil, errors.New("start it with `quipu ui`")
	}
	token := u.Query().Get("t")
	if token == "" {
		return nil, errors.New("the server address has no token")
	}
	target := &url.URL{Scheme: "http", Host: u.Host}
	return &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)
			r.Out.Host = target.Host
			r.Out.Header.Del("Cookie")
			r.Out.Header.Del("Referer")
			r.Out.AddCookie(&http.Cookie{Name: "quipu_token", Value: token}) // ui.cookieName
			if r.In.Method != http.MethodGet {
				r.Out.Header.Set("Origin", "http://"+target.Host)
			}
		},
	}, nil
}
