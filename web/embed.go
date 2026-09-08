// Package web embeds the dashboard's static assets so the Go binary serves the whole
// application — API and UI — from a single deployable artifact.
package web

import "embed"

//go:embed index.html style.css app.js
var FS embed.FS
