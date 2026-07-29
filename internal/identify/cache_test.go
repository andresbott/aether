package identify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andresbott/aether/libs/fpcalc"
)

// writeAudioFile creates a stand-in for an audio file. Only its stat metadata
// matters here: the fake fpcalc never reads it.
func writeAudioFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// countingAcoustID serves one fixed match and counts the lookups, which is how
// these tests observe whether a call reached the service or the cache answered.
func countingAcoustID(t *testing.T, calls *atomic.Int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{
			"status": "ok",
			"results": [{
				"score": 0.97,
				"recordings": [{"id": "rec-mbid-1", "title": "Song One"}]
			}]
		}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestCacheServesASecondLookupOfTheSameFile(t *testing.T) {
	var calls atomic.Int32
	srv := countingAcoustID(t, &calls)
	file := writeAudioFile(t, "song.mp3", "audio")
	bin := writeFakeFpcalc(t, `{"duration": 245.7, "fingerprint": "ABC123"}`)

	id := New(fpcalc.New(bin), newAcoustClient(srv))
	id.Cache = NewCache(10)

	first, dur, err := id.IdentifyFileWithDuration(context.Background(), file)
	if err != nil {
		t.Fatalf("first lookup: %v", err)
	}
	second, dur2, err := id.IdentifyFileWithDuration(context.Background(), file)
	if err != nil {
		t.Fatalf("second lookup: %v", err)
	}

	if got := calls.Load(); got != 1 {
		t.Fatalf("expected 1 AcoustID call, got %d", got)
	}
	if dur != 245.7 || dur2 != 245.7 {
		t.Fatalf("expected the cached duration to match, got %v and %v", dur, dur2)
	}
	if len(second) != len(first) || second[0].MBID != first[0].MBID {
		t.Fatalf("cached recordings differ: %+v vs %+v", second, first)
	}
}

// The whole point of the shared cache: the album flow calls the same primitive,
// so a per-track run already paid for the fingerprint pass.
func TestCacheIsSharedAcrossCallers(t *testing.T) {
	var calls atomic.Int32
	srv := countingAcoustID(t, &calls)
	file := writeAudioFile(t, "song.mp3", "audio")
	bin := writeFakeFpcalc(t, `{"duration": 100, "fingerprint": "ABC"}`)

	id := New(fpcalc.New(bin), newAcoustClient(srv))
	id.Cache = NewCache(10)

	// IdentifyFile is what /metadata/identify uses; IdentifyFileWithDuration is
	// what albumidentify uses. One cache must serve both.
	if _, err := id.IdentifyFile(context.Background(), file); err != nil {
		t.Fatalf("IdentifyFile: %v", err)
	}
	if _, _, err := id.IdentifyFileWithDuration(context.Background(), file); err != nil {
		t.Fatalf("IdentifyFileWithDuration: %v", err)
	}

	if got := calls.Load(); got != 1 {
		t.Fatalf("expected the second caller to hit the cache, got %d calls", got)
	}
}

func TestCacheMissesAfterTheFileChanges(t *testing.T) {
	var calls atomic.Int32
	srv := countingAcoustID(t, &calls)
	file := writeAudioFile(t, "song.mp3", "audio")
	bin := writeFakeFpcalc(t, `{"duration": 100, "fingerprint": "ABC"}`)

	id := New(fpcalc.New(bin), newAcoustClient(srv))
	id.Cache = NewCache(10)

	if _, err := id.IdentifyFile(context.Background(), file); err != nil {
		t.Fatalf("first lookup: %v", err)
	}

	// Rewrite the file with different content, so both size and mtime move.
	// A re-encoded or replaced file is a different recording as far as a
	// fingerprint is concerned, so the cached answer must not be reused.
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(file, []byte("different audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := id.IdentifyFile(context.Background(), file); err != nil {
		t.Fatalf("second lookup: %v", err)
	}

	if got := calls.Load(); got != 2 {
		t.Fatalf("expected the changed file to be looked up again, got %d calls", got)
	}
}

// A tag save rewrites the file, moving mtime without changing the audio. That is
// a deliberate miss: correctness over reuse, since size+mtime cannot tell a tag
// rewrite from a re-encode.
func TestCacheMissesAfterOnlyTheModTimeChanges(t *testing.T) {
	var calls atomic.Int32
	srv := countingAcoustID(t, &calls)
	file := writeAudioFile(t, "song.mp3", "audio")
	bin := writeFakeFpcalc(t, `{"duration": 100, "fingerprint": "ABC"}`)

	id := New(fpcalc.New(bin), newAcoustClient(srv))
	id.Cache = NewCache(10)

	if _, err := id.IdentifyFile(context.Background(), file); err != nil {
		t.Fatalf("first lookup: %v", err)
	}
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(file, future, future); err != nil {
		t.Fatal(err)
	}
	if _, err := id.IdentifyFile(context.Background(), file); err != nil {
		t.Fatalf("second lookup: %v", err)
	}

	if got := calls.Load(); got != 2 {
		t.Fatalf("expected a touched file to be looked up again, got %d calls", got)
	}
}

func TestCacheKeepsFilesApart(t *testing.T) {
	var calls atomic.Int32
	srv := countingAcoustID(t, &calls)
	dir := t.TempDir()
	one := filepath.Join(dir, "one.mp3")
	two := filepath.Join(dir, "two.mp3")
	for _, p := range []string{one, two} {
		if err := os.WriteFile(p, []byte("audio"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	bin := writeFakeFpcalc(t, `{"duration": 100, "fingerprint": "ABC"}`)

	id := New(fpcalc.New(bin), newAcoustClient(srv))
	id.Cache = NewCache(10)

	if _, err := id.IdentifyFile(context.Background(), one); err != nil {
		t.Fatalf("one: %v", err)
	}
	// Same size and (very likely) same mtime as the first file: only the path
	// tells them apart, so a key that ignored it would serve one file's
	// recordings for the other.
	if _, err := id.IdentifyFile(context.Background(), two); err != nil {
		t.Fatalf("two: %v", err)
	}

	if got := calls.Load(); got != 2 {
		t.Fatalf("expected two distinct files to be looked up separately, got %d calls", got)
	}
}

// A failure must not be cached: an AcoustID outage or a rate-limited lookup has
// to be retryable, or one bad moment would poison the file until restart.
func TestCacheDoesNotStoreFailures(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"status":"error","error":{"message":"rate limit"}}`))
	}))
	defer srv.Close()
	file := writeAudioFile(t, "song.mp3", "audio")
	bin := writeFakeFpcalc(t, `{"duration": 100, "fingerprint": "ABC"}`)

	id := New(fpcalc.New(bin), newAcoustClient(srv))
	id.Cache = NewCache(10)

	if _, err := id.IdentifyFile(context.Background(), file); err == nil {
		t.Fatal("expected the first lookup to fail")
	}
	if _, err := id.IdentifyFile(context.Background(), file); err == nil {
		t.Fatal("expected the second lookup to fail")
	}

	if got := calls.Load(); got != 2 {
		t.Fatalf("expected a failed lookup to be retried, got %d calls", got)
	}
}

// "This file matches nothing" is a real answer and the most expensive kind to
// re-derive, so unlike a failure it IS cached.
func TestCacheStoresAnEmptyMatch(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"status": "ok", "results": []}`))
	}))
	defer srv.Close()
	file := writeAudioFile(t, "song.mp3", "audio")
	bin := writeFakeFpcalc(t, `{"duration": 100, "fingerprint": "ABC"}`)

	id := New(fpcalc.New(bin), newAcoustClient(srv))
	id.Cache = NewCache(10)

	for i := 0; i < 2; i++ {
		recs, err := id.IdentifyFile(context.Background(), file)
		if err != nil {
			t.Fatalf("lookup %d: %v", i, err)
		}
		if len(recs) != 0 {
			t.Fatalf("expected no recordings, got %d", len(recs))
		}
	}

	if got := calls.Load(); got != 1 {
		t.Fatalf("expected an empty match to be cached, got %d calls", got)
	}
}

// A file that cannot be stat'd (already gone, or a path the caller invented)
// simply is not cacheable — identification still runs, it just is not stored.
func TestCacheBypassedWhenTheFileCannotBeStatted(t *testing.T) {
	var calls atomic.Int32
	srv := countingAcoustID(t, &calls)
	bin := writeFakeFpcalc(t, `{"duration": 100, "fingerprint": "ABC"}`)

	id := New(fpcalc.New(bin), newAcoustClient(srv))
	id.Cache = NewCache(10)

	missing := filepath.Join(t.TempDir(), "not-there.mp3")
	for i := 0; i < 2; i++ {
		if _, err := id.IdentifyFile(context.Background(), missing); err != nil {
			t.Fatalf("lookup %d: %v", i, err)
		}
	}

	if got := calls.Load(); got != 2 {
		t.Fatalf("expected an unstattable path to bypass the cache, got %d calls", got)
	}
}

// A nil cache is the default: identification works exactly as before.
func TestNilCacheIdentifiesEveryTime(t *testing.T) {
	var calls atomic.Int32
	srv := countingAcoustID(t, &calls)
	file := writeAudioFile(t, "song.mp3", "audio")
	bin := writeFakeFpcalc(t, `{"duration": 100, "fingerprint": "ABC"}`)

	id := New(fpcalc.New(bin), newAcoustClient(srv))

	for i := 0; i < 2; i++ {
		if _, err := id.IdentifyFile(context.Background(), file); err != nil {
			t.Fatalf("lookup %d: %v", i, err)
		}
	}

	if got := calls.Load(); got != 2 {
		t.Fatalf("expected no caching without a cache, got %d calls", got)
	}
}

func TestCacheEvictsTheLeastRecentlyUsedEntry(t *testing.T) {
	var calls atomic.Int32
	srv := countingAcoustID(t, &calls)
	dir := t.TempDir()
	paths := make([]string, 3)
	for i := range paths {
		paths[i] = filepath.Join(dir, string(rune('a'+i))+".mp3")
		// Distinct sizes so each file is unmistakably its own entry.
		if err := os.WriteFile(paths[i], []byte(string(make([]byte, i+1))), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	bin := writeFakeFpcalc(t, `{"duration": 100, "fingerprint": "ABC"}`)

	id := New(fpcalc.New(bin), newAcoustClient(srv))
	id.Cache = NewCache(2)

	// Fill the cache, then read the oldest so it is no longer least-recently-used.
	for _, p := range paths[:2] {
		if _, err := id.IdentifyFile(context.Background(), p); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := id.IdentifyFile(context.Background(), paths[0]); err != nil {
		t.Fatal(err)
	}
	// A third file evicts paths[1], the least recently used.
	if _, err := id.IdentifyFile(context.Background(), paths[2]); err != nil {
		t.Fatal(err)
	}
	before := calls.Load()
	if _, err := id.IdentifyFile(context.Background(), paths[0]); err != nil {
		t.Fatal(err)
	}
	if got := calls.Load(); got != before {
		t.Fatalf("expected the touched entry to survive eviction, got %d extra calls", got-before)
	}
	if _, err := id.IdentifyFile(context.Background(), paths[1]); err != nil {
		t.Fatal(err)
	}
	if got := calls.Load(); got != before+1 {
		t.Fatalf("expected the evicted entry to be looked up again, got %d calls", got)
	}
}

func TestCacheIsSafeForConcurrentUse(t *testing.T) {
	var calls atomic.Int32
	srv := countingAcoustID(t, &calls)
	file := writeAudioFile(t, "song.mp3", "audio")
	bin := writeFakeFpcalc(t, `{"duration": 100, "fingerprint": "ABC"}`)

	id := New(fpcalc.New(bin), newAcoustClient(srv))
	id.Cache = NewCache(10)

	// The handlers fingerprint a selection in a loop today, but nothing stops a
	// future caller from parallelising it; -race would catch an unguarded map.
	done := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func() {
			_, err := id.IdentifyFile(context.Background(), file)
			done <- err
		}()
	}
	for i := 0; i < 8; i++ {
		if err := <-done; err != nil {
			t.Fatalf("concurrent lookup: %v", err)
		}
	}
}
