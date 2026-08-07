// Command devproxy is a smoke-testing stand-in for an authenticating reverse
// proxy (Authelia behind Traefik/nginx). It forwards everything to a local
// aether and behaves like a correctly configured deployment:
//
//   - inbound identity headers are ALWAYS stripped (a client must never be
//     able to smuggle them),
//   - the configured identity is injected on every request EXCEPT /rest,
//     which is bypassed exactly like the real proxy ACL (Subsonic clients
//     authenticate with their apiKey, not headers).
//
// Every request is logged with the identity the proxy injected (or "bypass"
// for /rest), the upstream status and the round-trip duration, so a smoke test
// shows at a glance which requests aether saw headers on.
//
// Run it via `make proxy USER=admin GROUP=aether-admin` and point a browser
// or curl at http://localhost:8076. Aether itself should run with
// Auth.Method: proxy-header.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

// sanitizeForLog strips CR/LF from a value that came off the wire before it
// reaches the access log: without it a request could embed newlines in its
// method or URI and forge additional log lines.
func sanitizeForLog(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' {
			return -1
		}
		return r
	}, s)
}

// logWriter records the status code so the access log can report it.
type logWriter struct {
	http.ResponseWriter
	status int
}

func (w *logWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *logWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func main() {
	user := flag.String("user", "", "value injected as the user header (required)")
	groups := flag.String("groups", "", "value injected as the groups header (optional, comma-separated)")
	userHeader := flag.String("user-header", "Remote-User", "user header name")
	groupsHeader := flag.String("groups-header", "Remote-Groups", "groups header name")
	listen := flag.String("listen", ":8076", "address the proxy listens on")
	// 127.0.0.1, not localhost: Go resolves localhost to ::1 first, so aether
	// would see the peer as ::1 and reject the headers unless ::1 is in
	// ProxyHeader.TrustedProxies — which looks exactly like a broken SPA.
	target := flag.String("target", "http://127.0.0.1:8075", "aether base URL")
	flag.Parse()

	if *user == "" {
		log.Fatal("devproxy: -user is required (make proxy USER=<login>)")
	}
	upstream, err := url.Parse(*target)
	if err != nil {
		log.Fatalf("devproxy: invalid target %q: %v", *target, err)
	}

	proxy := httputil.NewSingleHostReverseProxy(upstream)
	director := proxy.Director
	proxy.Director = func(r *http.Request) {
		director(r)
		// A real proxy strips inbound identity headers unconditionally.
		r.Header.Del(*userHeader)
		r.Header.Del(*groupsHeader)
		// /rest is bypassed: Subsonic clients hold no proxy session, so the
		// proxy injects nothing there and aether must ignore headers anyway.
		if strings.HasPrefix(r.URL.Path, "/rest") {
			return
		}
		r.Header.Set(*userHeader, *user)
		if *groups != "" {
			r.Header.Set(*groupsHeader, *groups)
		}
	}

	identity := *user
	if *groups != "" {
		identity += " (" + *groups + ")"
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		injected := identity
		if strings.HasPrefix(r.URL.Path, "/rest") {
			injected = "bypass"
		}
		lw := &logWriter{ResponseWriter: w}
		start := time.Now()
		proxy.ServeHTTP(lw, r)
		//nolint:gosec // G706: both wire-derived values go through sanitizeForLog, which drops CR/LF; gosec's taint analysis does not see through strings.Map
		log.Printf("%s %s -> %d as %s in %s",
			sanitizeForLog(r.Method), sanitizeForLog(r.URL.RequestURI()),
			lw.status, injected, time.Since(start).Round(time.Millisecond))
	})

	log.SetFlags(log.Ltime)
	fmt.Printf("devproxy: http://localhost%s -> %s as %s [%s/%s], /rest bypassed\n",
		*listen, *target, identity, *userHeader, *groupsHeader)
	// Explicit timeouts rather than http.ListenAndServe's none. Generous because
	// this proxies audio streams: a long download must not be cut mid-song, so
	// only the header read is tightly bounded.
	srv := &http.Server{
		Addr:              *listen,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	log.Fatal(srv.ListenAndServe())
}
