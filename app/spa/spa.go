package spa

import (
	"embed"
	"mime"
	"net/http"

	handlers "github.com/go-bumbu/http/handlers/spa"
)

//go:embed files/ui/*
var UiFiles embed.FS

func init() {
	// net/http serves the embedded SPA through mime.TypeByExtension, whose
	// built-in table has no .webmanifest entry — the file then falls through to
	// content sniffing and goes out as text/plain, which browsers refuse to
	// treat as a web app manifest. /etc/mime.types would cover it on some
	// distros but not in a scratch container, so register it explicitly.
	if err := mime.AddExtensionType(".webmanifest", "application/manifest+json"); err != nil {
		panic(err)
	}
}

func App(path string) (http.Handler, error) {
	return handlers.NewSpaHAndler(
		UiFiles,
		"files/ui",
		path,
	)
}
