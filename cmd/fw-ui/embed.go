package main

import "embed"

// staticFS holds the built frontend (Svelte + DaisyUI). During development the
// placeholder index.html is served; `npm run build` in web/ writes the real
// assets into static/.
//
//go:embed static
var staticFS embed.FS
