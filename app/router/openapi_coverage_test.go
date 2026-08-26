package router

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/andresbott/aether/internal/identify"
	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/tags"
	"github.com/andresbott/aether/internal/taskrunner"
	"github.com/glebarez/sqlite"
	"github.com/go-bumbu/userauth/auth/cookieauth"
	"github.com/go-bumbu/userauth/service/pat"
	"github.com/go-bumbu/userauth/userstore/userdb"
	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

// noopTagReader satisfies tags.Reader without touching disk. attachApiV1
// only checks the field for nilness to decide whether to mount /metadata; a
// tag read is never exercised by this test.
type noopTagReader struct{}

func (noopTagReader) CanRead(string) bool { return false }

func (noopTagReader) Read(context.Context, string) (tags.Metadata, error) {
	return tags.Metadata{}, nil
}

// newMaximalAPIV1Router builds a MainAppHandler with every conditional
// /api/v1 mount active — store, taskRunner, users, sessions, tagReader and
// identifier all non-nil, native auth mode — and returns a fresh router with
// only attachApiV1's surface mounted on it (no /rest, no SPA), so nothing
// needs filtering before comparing against the spec. Auth wiring mirrors
// newNativeAuthRouter (auth_test.go); the fields beyond that are exactly the
// ones attachApiV1 gates a mount on (api_v1.go) — every other MainAppHandler
// field is assigned into a handler struct but never checked for nilness
// during route registration, so it is left at its zero value.
func newMaximalAPIV1Router(t *testing.T) *mux.Router {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatal(err)
	}
	users, err := userdb.New(db, userdb.Opts{BcryptDifficulty: bcrypt.MinCost, DefaultEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	cookieStore, err := cookieauth.NewCookieStore(make([]byte, 64), make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	cookieStore.Options.Secure = false
	sessions, err := cookieauth.New(cookieauth.Cfg{
		Store:         cookieStore,
		SessionDur:    time.Hour,
		MaxSessionDur: 24 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	cipher, err := pat.NewAESGCMCipher(bytes.Repeat([]byte{0x42}, 32), "k1")
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := pat.NewService(users.PATStore(), users, pat.Opts{Prefix: "aether", Cipher: cipher})
	if err != nil {
		t.Fatal(err)
	}
	runner, err := taskrunner.NewRunner(taskrunner.Cfg{DB: db})
	if err != nil {
		t.Fatal(err)
	}

	h := &MainAppHandler{
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		authMethod: "native",
		users:      users,
		sessions:   sessions,
		tokens:     tokens,
		store:      store.New(db),
		taskRunner: runner,
		tagReader:  noopTagReader{},
		identifier: identify.New(nil, nil),
	}
	r := mux.NewRouter()
	h.attachApiV1(r.PathPrefix(apiV1MountPrefix).Subrouter())
	return r
}

// apiV1Route is one HTTP operation, mount-relative to /api/v1 — e.g.
// {Method: "GET", Path: "/libraries/{id}"} — the shape both the mounted
// router and the OpenAPI spec are reduced to for comparison.
type apiV1Route struct {
	Method string
	Path   string
}

func (o apiV1Route) String() string {
	return o.Method + " " + o.Path
}

// muxVarPattern strips a mux path variable's inline regexp ("{id:[0-9]+}" ->
// "{id}"), the one syntax difference between a mux template and an OpenAPI
// path: OpenAPI has no equivalent constraint syntax on the path itself.
var muxVarPattern = regexp.MustCompile(`\{([^:}]+):[^}]*\}`)

// walkMaximalAPIV1Routes walks root — which, per newMaximalAPIV1Router,
// carries only attachApiV1's mounts under /api/v1 — and returns every
// (method, path) it finds, mount-relative and with mux vars normalized to
// OpenAPI's bare {var} form. The wrong-api-call catch-all (api_v1.go,
// r.PathPrefix("").HandlerFunc(...)) binds no HTTP method, which is exactly
// how it is excluded here: it answers "wrong api call" for anything
// unmatched and describes no operation. The mount point itself (the
// PathPrefix(apiV1MountPrefix) route) is excluded the same way.
func walkMaximalAPIV1Routes(t *testing.T, root *mux.Router) map[apiV1Route]bool {
	t.Helper()
	routes := map[apiV1Route]bool{}
	err := root.Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		tmpl, err := route.GetPathTemplate()
		if err != nil {
			return nil
		}
		methods, err := route.GetMethods()
		if err != nil {
			return nil // no bound method: the catch-all or the mount point itself
		}
		relPath := strings.TrimPrefix(tmpl, apiV1MountPrefix)
		relPath = muxVarPattern.ReplaceAllString(relPath, "{$1}")
		for _, m := range methods {
			routes[apiV1Route{Method: m, Path: relPath}] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking /api/v1 router: %v", err)
	}
	return routes
}

// specHTTPMethods are the OpenAPI path-item keys that describe an operation.
// Every other key on a path item ("parameters", "summary", ...) is path-level
// metadata, not an operation, and must not be counted as one.
var specHTTPMethods = map[string]bool{
	"get": true, "put": true, "post": true, "delete": true,
	"options": true, "head": true, "patch": true, "trace": true,
}

// loadSpecOperations parses docs/openapi/aether-v1.yaml's "paths" into the
// same (method, path) shape walkMaximalAPIV1Routes produces. It reads the
// plain YAML structure — via yaml.v3, already a dependency in go.mod, no new
// one added — rather than a full OpenAPI loader (kin-openapi is not a
// dependency of this project at all): this test checks surface coverage, not
// schema fidelity, so only the path and method keys matter.
func loadSpecOperations(t *testing.T) map[apiV1Route]bool {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not resolve this test file's own location")
	}
	// Resolved relative to this source file, not the "go test" working
	// directory, so the path holds regardless of how the test is invoked.
	specPath := filepath.Join(filepath.Dir(thisFile), "..", "..", "docs", "openapi", "aether-v1.yaml")
	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("reading %s: %v", specPath, err)
	}

	var doc struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parsing %s: %v", specPath, err)
	}

	routes := map[apiV1Route]bool{}
	for path, ops := range doc.Paths {
		for method := range ops {
			if !specHTTPMethods[method] {
				continue
			}
			routes[apiV1Route{Method: strings.ToUpper(method), Path: path}] = true
		}
	}
	return routes
}

// TestOpenAPICoversAllV1Routes is the anti-drift keystone for
// docs/openapi/aether-v1.yaml (design spec
// docs/superpowers/specs/2026-08-26-openapi-v1-full-coverage.md, §D2): it
// asserts the spec and the routes attachApiV1 mounts describe exactly the
// same /api/v1 surface, in both directions. It runs against a maximal router
// — every conditional mount active — because this test's job is to catch a
// spec that fell behind the code, or the reverse, not to describe any one
// deployment's actual feature set.
func TestOpenAPICoversAllV1Routes(t *testing.T) {
	root := newMaximalAPIV1Router(t)
	mounted := walkMaximalAPIV1Routes(t, root)
	specced := loadSpecOperations(t)

	var missingFromSpec []string
	for r := range mounted {
		if !specced[r] {
			missingFromSpec = append(missingFromSpec, r.String())
		}
	}
	sort.Strings(missingFromSpec)

	var missingFromRouter []string
	for r := range specced {
		if !mounted[r] {
			missingFromRouter = append(missingFromRouter, r.String())
		}
	}
	sort.Strings(missingFromRouter)

	if len(missingFromSpec) > 0 {
		t.Errorf("routes mounted on /api/v1 but missing from the OpenAPI spec:\n%s",
			strings.Join(missingFromSpec, "\n"))
	}
	if len(missingFromRouter) > 0 {
		t.Errorf("OpenAPI operations with no mounted /api/v1 route:\n%s",
			strings.Join(missingFromRouter, "\n"))
	}
}
