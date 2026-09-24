package spa

import (
	"embed"
	"mime"
	"net/http"

	spah "github.com/go-bumbu/http/spa"
)

//go:embed all:files/ui
var UiFiles embed.FS

func init() {
	// net/http serves the embedded SPA through mime.TypeByExtension, whose
	// built-in table has neither a .webmanifest nor a .ico entry — those files
	// then fall through to content sniffing and go out as text/plain resp.
	// application/octet-stream, which browsers refuse to treat as a web app
	// manifest resp. an icon. /etc/mime.types would cover them on some distros
	// but not in a scratch container, so register them explicitly.
	for ext, typ := range map[string]string{
		".webmanifest": "application/manifest+json",
		".ico":         "image/vnd.microsoft.icon",
	} {
		if err := mime.AddExtensionType(ext, typ); err != nil {
			panic(err)
		}
	}
}

func App(path string) (http.Handler, error) {
	return spah.New(spah.Cfg{
		FS:         UiFiles,
		SubDir:     "files/ui",
		PathPrefix: path,
	})
}
