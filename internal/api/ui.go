//go:build !no_api

package api

import (
	"embed"
)

//go:embed static/*
var staticFS embed.FS

// getDashboardHTML returns the embedded dashboard HTML page.
func getDashboardHTML() []byte {
	content, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		return []byte("<!DOCTYPE html><html><body><h1>GSM2MQTT Dashboard</h1></body></html>")
	}
	return content
}

// getFaviconSVG returns the embedded SVG favicon.
func getFaviconSVG() []byte {
	content, err := staticFS.ReadFile("static/favicon.svg")
	if err != nil {
		return []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><text y=".9em" font-size="90">📟</text></svg>`)
	}
	return content
}
