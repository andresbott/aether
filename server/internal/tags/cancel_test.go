package tags_test

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"

	"github.com/andresbott/aether/internal/tags"
)

// TestFFProbeReader_ReadCanceledContext asserts a canceled caller stops the read
// instead of spawning ffprobe and waiting for it. Before Read took a context the
// exec was unbounded and uninterruptible: an abandoned scan or request kept
// reading files to the end.
func TestFFProbeReader_ReadCanceledContext(t *testing.T) {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not installed")
	}
	dst := writeTaggedFLAC(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := tags.FFProbeReader{}.Read(ctx, dst)
	if err == nil {
		t.Fatal("expected an error for a canceled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error should wrap context.Canceled, got %v", err)
	}
}

// TestFFProbeReader_ReadExpiredContext covers the deadline path, which reports
// DeadlineExceeded rather than Canceled — the distinction a scan's error list
// surfaces to explain why a file was skipped.
func TestFFProbeReader_ReadExpiredContext(t *testing.T) {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not installed")
	}
	dst := writeTaggedFLAC(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond) // let the deadline pass

	_, err := tags.FFProbeReader{}.Read(ctx, dst)
	if err == nil {
		t.Fatal("expected an error for an expired context")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error should wrap context.DeadlineExceeded, got %v", err)
	}
}

// TestFFProbeTimeoutIsBounded documents the standalone timeout. It is what saves
// a scheduled scan, which has no caller to cancel it: cancellation alone would
// leave a wedged ffprobe holding a scan worker forever.
func TestFFProbeTimeoutIsBounded(t *testing.T) {
	if tags.FFProbeTimeout <= 0 {
		t.Fatal("FFProbeTimeout must be positive so a wedged ffprobe cannot hang a scan")
	}
}

// TestTaglibReader_ReadCanceledContext asserts the taglib reader checks the
// context up front. It cannot interrupt a cgo call already under way, but an
// abandoned scan must not start new reads.
func TestTaglibReader_ReadCanceledContext(t *testing.T) {
	dst := writeTaggedFLAC(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := tags.TaglibReader{}.Read(ctx, dst)
	if err == nil {
		t.Fatal("expected an error for a canceled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error should wrap context.Canceled, got %v", err)
	}
}

// ctxRecordingReader records the context each Read receives, so a test can
// assert the caller's context reaches the reader rather than a detached one.
type ctxRecordingReader struct {
	canRead bool
	err     error
	gotCtx  []context.Context
}

func (r *ctxRecordingReader) CanRead(string) bool { return r.canRead }

func (r *ctxRecordingReader) Read(ctx context.Context, _ string) (tags.Metadata, error) {
	r.gotCtx = append(r.gotCtx, ctx)
	return tags.Metadata{}, r.err
}

// TestFallbackReader_PropagatesContext asserts FallbackReader hands the caller's
// context to the reader it delegates to, rather than swallowing it.
func TestFallbackReader_PropagatesContext(t *testing.T) {
	primary := &ctxRecordingReader{canRead: true}
	fallback := &ctxRecordingReader{canRead: true}
	r := tags.NewFallbackReader(primary, fallback)

	type ctxKey struct{}
	ctx := context.WithValue(context.Background(), ctxKey{}, "marker")

	if _, err := r.Read(ctx, "song.flac"); err != nil {
		t.Fatal(err)
	}
	if len(primary.gotCtx) != 1 {
		t.Fatalf("primary Read called %d times, want 1", len(primary.gotCtx))
	}
	if primary.gotCtx[0].Value(ctxKey{}) != "marker" {
		t.Error("primary did not receive the caller's context")
	}
}

// TestFallbackReader_CanceledContextSkipsFallback asserts a canceled context
// short-circuits the second attempt. The fallback exists for files the primary
// cannot parse; spending it on a read the caller already abandoned would double
// the work known to be unwanted.
func TestFallbackReader_CanceledContextSkipsFallback(t *testing.T) {
	primary := &ctxRecordingReader{canRead: true, err: errors.New("primary failed")}
	fallback := &ctxRecordingReader{canRead: true}
	r := tags.NewFallbackReader(primary, fallback)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := r.Read(ctx, "song.flac"); err == nil {
		t.Fatal("expected the primary's error to surface")
	}
	if len(fallback.gotCtx) != 0 {
		t.Errorf("fallback was consulted %d times despite a canceled context", len(fallback.gotCtx))
	}
}

// TestFallbackReader_PrimaryErrorStillFallsBack guards the regression risk of the
// check above: a live context must keep the existing fallback behaviour.
func TestFallbackReader_PrimaryErrorStillFallsBack(t *testing.T) {
	primary := &ctxRecordingReader{canRead: true, err: errors.New("primary failed")}
	fallback := &ctxRecordingReader{canRead: true}
	r := tags.NewFallbackReader(primary, fallback)

	if _, err := r.Read(context.Background(), "song.flac"); err != nil {
		t.Fatalf("expected the fallback to succeed, got %v", err)
	}
	if len(fallback.gotCtx) != 1 {
		t.Errorf("fallback consulted %d times, want 1", len(fallback.gotCtx))
	}
}
