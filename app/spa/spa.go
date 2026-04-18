package spa

import (
	"embed"
	"net/http"

	handlers "github.com/go-bumbu/http/handlers/spa"
)

//go:embed files/ui/*
var UiFiles embed.FS

func App(path string) (http.Handler, error) {
	return handlers.NewSpaHAndler(
		UiFiles,
		"files/ui",
		path,
	)
}
