// Package webbrowser ports Agno's WebBrowserTools: open a URL in the local
// user's default web browser (xdg-open on Linux/BSD, open on macOS,
// rundll32 url.dll on Windows).
package webbrowser

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// Opener opens url in a browser.
type Opener func(url string, newWindow bool) error

// WebBrowserTools opens pages in the local browser.
type WebBrowserTools struct {
	toolkit.Toolkit
	enableOpenPage bool
	all            bool
	opener         Opener
}

// Option configures WebBrowserTools.
type Option func(*WebBrowserTools)

// WithEnableOpenPage toggles OpenPage (default true).
func WithEnableOpenPage(b bool) Option { return func(w *WebBrowserTools) { w.enableOpenPage = b } }

// WithAll enables every tool.
func WithAll() Option { return func(w *WebBrowserTools) { w.all = true } }

// WithOpener replaces the function used to launch the browser (useful in tests).
func WithOpener(o Opener) Option { return func(w *WebBrowserTools) { w.opener = o } }

// OpenPageParams are the parameters of OpenPage.
type OpenPageParams struct {
	URL       string `json:"url" description:"URL to open." required:"true"`
	NewWindow bool   `json:"new_window,omitempty" description:"If true, open in a new window, otherwise open in a new tab. Default is false."`
}

// New creates the web browser toolkit.
func New(opts ...Option) *WebBrowserTools {
	w := &WebBrowserTools{enableOpenPage: true, opener: SystemOpener}
	w.Toolkit = toolkit.NewToolkit()
	w.Toolkit.Name = "webbrowser_tools"
	w.Toolkit.Description = "Open pages in the local web browser."
	for _, o := range opts {
		o(w)
	}
	if w.all || w.enableOpenPage {
		w.Toolkit.Register("OpenPage", "Open a URL in a browser window or tab.", w, w.OpenPage, OpenPageParams{})
	}
	return w
}

// SystemOpener launches the platform's default browser. The new-window hint
// is not supported by the system launchers and is ignored.
func SystemOpener(url string, _ bool) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// OpenPage opens url in the browser.
func (w *WebBrowserTools) OpenPage(params OpenPageParams) (interface{}, error) {
	url := strings.TrimSpace(params.URL)
	if url == "" || strings.HasPrefix(url, "-") {
		return nil, fmt.Errorf("invalid URL: %q", params.URL)
	}
	if err := w.opener(url, params.NewWindow); err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", url, err)
	}
	return fmt.Sprintf("Opened %s in the web browser", url), nil
}

// Execute implements toolkit.Tool.
func (w *WebBrowserTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return w.Toolkit.Execute(methodName, input)
}
