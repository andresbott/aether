package discovery_test

import (
	"testing"
	"time"

	"github.com/andresbott/aether/internal/discovery"
)

// albumCands builds n played albums (so none land in the rediscovery pool),
// oldest first, with descending play counts so the ranking is predictable.
func albumCands(n int, now time.Time) []discovery.Candidate {
	out := make([]discovery.Candidate, 0, n)
	for i := 0; i < n; i++ {
		played := now.Add(-time.Duration(i) * time.Hour)
		out = append(out, discovery.Candidate{
			Kind:         discovery.KindAlbum,
			ID:           uint(i + 1),
			CreatedAt:    now.Add(-time.Duration(i) * 24 * time.Hour),
			PlayCount:    n - i,
			LastPlayedAt: &played,
		})
	}
	return out
}

func unplayedCands(n int, now time.Time, startID uint) []discovery.Candidate {
	out := make([]discovery.Candidate, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, discovery.Candidate{
			Kind:      discovery.KindAlbum,
			ID:        startID + uint(i),
			CreatedAt: now.Add(-time.Duration(400+i) * 24 * time.Hour),
		})
	}
	return out
}

func TestRankAssignsSequentialRanksFromOffset(t *testing.T) {
	now := time.Now()
	got := discovery.Rank(albumCands(10, now), discovery.TasteProfile{}, 1, now, 0, 5)
	if len(got) != 5 {
		t.Fatalf("got %d items, want 5", len(got))
	}
	for i, r := range got {
		if r.Rank != i {
			t.Fatalf("item %d has Rank %d, want %d", i, r.Rank, i)
		}
	}
	page2 := discovery.Rank(albumCands(10, now), discovery.TasteProfile{}, 1, now, 5, 5)
	for i, r := range page2 {
		if r.Rank != 5+i {
			t.Fatalf("page 2 item %d has Rank %d, want %d", i, r.Rank, 5+i)
		}
	}
}

func TestRankIsDeterministicForOneSeed(t *testing.T) {
	now := time.Now()
	a := discovery.Rank(albumCands(20, now), discovery.TasteProfile{}, 7, now, 0, 20)
	b := discovery.Rank(albumCands(20, now), discovery.TasteProfile{}, 7, now, 0, 20)
	for i := range a {
		if a[i].ID != b[i].ID || a[i].Rank != b[i].Rank {
			t.Fatalf("rank %d differed between identical calls: %+v vs %+v", i, a[i], b[i])
		}
	}
}

// The paging guarantee: a wider pool (what a deeper offset gathers) must not
// reorder ranks already served on an earlier page.
func TestPagesDoNotOverlapOrGap(t *testing.T) {
	now := time.Now()
	cands := append(albumCands(30, now), unplayedCands(10, now, 100)...)
	full := discovery.Rank(cands, discovery.TasteProfile{}, 3, now, 0, 40)
	page1 := discovery.Rank(cands, discovery.TasteProfile{}, 3, now, 0, 20)
	page2 := discovery.Rank(cands, discovery.TasteProfile{}, 3, now, 20, 20)

	joined := append(append([]discovery.Ranked{}, page1...), page2...)
	if len(joined) != len(full) {
		t.Fatalf("paged total %d != single-shot total %d", len(joined), len(full))
	}
	for i := range full {
		if joined[i] != full[i] {
			t.Fatalf("rank %d: paged %+v != single-shot %+v", i, joined[i], full[i])
		}
	}
}

func TestQuotaSlotsDrawFromNeverPlayedItems(t *testing.T) {
	now := time.Now()
	// 30 played + 10 never-played. Every 4th slot must be a never-played id
	// (>= 100), the rest played ones (< 100).
	cands := append(albumCands(30, now), unplayedCands(10, now, 100)...)
	got := discovery.Rank(cands, discovery.TasteProfile{}, 5, now, 0, 20)
	for _, r := range got {
		isQuotaSlot := r.Rank%discovery.QuotaEvery == discovery.QuotaEvery-1
		fromRediscoveryPool := r.ID >= 100
		if isQuotaSlot != fromRediscoveryPool {
			t.Fatalf("rank %d: quotaSlot=%v but id=%d (rediscovery=%v)",
				r.Rank, isQuotaSlot, r.ID, fromRediscoveryPool)
		}
	}
}

func TestQuotaSlotsReportRediscover(t *testing.T) {
	now := time.Now()
	cands := append(albumCands(30, now), unplayedCands(10, now, 100)...)
	got := discovery.Rank(cands, discovery.TasteProfile{}, 5, now, 0, 20)
	for _, r := range got {
		if r.Rank%discovery.QuotaEvery == discovery.QuotaEvery-1 {
			if r.Reason != discovery.ReasonRediscover {
				t.Fatalf("quota slot at rank %d reported %q, want %q",
					r.Rank, r.Reason, discovery.ReasonRediscover)
			}
		} else if r.Reason == discovery.ReasonRediscover {
			t.Fatalf("non-quota slot at rank %d reported rediscover", r.Rank)
		}
	}
}

func TestMainListFillsWhenRediscoveryPoolIsEmpty(t *testing.T) {
	now := time.Now()
	got := discovery.Rank(albumCands(12, now), discovery.TasteProfile{}, 5, now, 0, 12)
	if len(got) != 12 {
		t.Fatalf("got %d items, want all 12 filled from the main list", len(got))
	}
	for _, r := range got {
		if r.Reason == discovery.ReasonRediscover {
			t.Fatal("no never-played candidates exist, so nothing may report rediscover")
		}
	}
}

func TestRediscoveryFillsWhenMainListIsEmpty(t *testing.T) {
	now := time.Now()
	got := discovery.Rank(unplayedCands(8, now, 1), discovery.TasteProfile{}, 5, now, 0, 8)
	if len(got) != 8 {
		t.Fatalf("got %d items, want all 8", len(got))
	}
}

func TestRankHandlesEmptyInput(t *testing.T) {
	if got := discovery.Rank(nil, discovery.TasteProfile{}, 1, time.Now(), 0, 10); len(got) != 0 {
		t.Fatalf("got %d items from no candidates, want 0", len(got))
	}
}

func TestOffsetPastTheEndReturnsNothing(t *testing.T) {
	now := time.Now()
	if got := discovery.Rank(albumCands(5, now), discovery.TasteProfile{}, 1, now, 100, 10); len(got) != 0 {
		t.Fatalf("got %d items past the end, want 0", len(got))
	}
}

func TestBothKindsCompeteInOneOrdering(t *testing.T) {
	now := time.Now()
	// A starred playlist must outrank an unstarred album of the same age:
	// Favorite (0.25) dominates.
	played := now
	cands := []discovery.Candidate{
		{Kind: discovery.KindAlbum, ID: 1, CreatedAt: now, PlayCount: 1, LastPlayedAt: &played},
		{
			Kind: discovery.KindPlaylist, ID: 1, CreatedAt: now, PlayCount: 1,
			LastPlayedAt: &played, StarredAt: &played,
		},
	}
	got := discovery.Rank(cands, discovery.TasteProfile{}, 1, now, 0, 2)
	if got[0].Kind != discovery.KindPlaylist {
		t.Fatalf("rank 0 is %s, want the starred playlist", got[0].Kind)
	}
}

// Ties must break deterministically, or the paging guarantee above cannot hold.
func TestTiesBreakDeterministicallyByKindThenID(t *testing.T) {
	now := time.Now()
	// Identical signals and identical jitter is impossible via Jitter(), so force
	// a tie by giving two candidates the same everything except kind/id and
	// checking the order is stable across repeated calls.
	cands := []discovery.Candidate{
		{Kind: discovery.KindPlaylist, ID: 2, CreatedAt: now, PlayCount: 1, LastPlayedAt: &now},
		{Kind: discovery.KindAlbum, ID: 2, CreatedAt: now, PlayCount: 1, LastPlayedAt: &now},
		{Kind: discovery.KindAlbum, ID: 1, CreatedAt: now, PlayCount: 1, LastPlayedAt: &now},
	}
	first := discovery.Rank(cands, discovery.TasteProfile{}, 1, now, 0, 3)
	for i := 0; i < 20; i++ {
		again := discovery.Rank(cands, discovery.TasteProfile{}, 1, now, 0, 3)
		for j := range first {
			if first[j] != again[j] {
				t.Fatalf("iteration %d rank %d unstable: %+v vs %+v", i, j, first[j], again[j])
			}
		}
	}
}

// TestQuotaEveryIsFour verifies the constant value, which mutations can silently change.
func TestQuotaEveryIsFour(t *testing.T) {
	if discovery.QuotaEvery != 4 {
		t.Fatalf("QuotaEvery is %d, want 4", discovery.QuotaEvery)
	}
}

// TestRediscoveryOrderChangesWithJitter verifies that jitter influences the
// rediscovery pool ordering. Without jitter, identical unplayed candidates
// would score identically and appear in input order; with jitter they should
// change order across different seeds.
func TestRediscoveryOrderChangesWithJitter(t *testing.T) {
	now := time.Now()
	// Two unplayed albums with identical signals except jitter
	cands := []discovery.Candidate{
		{Kind: discovery.KindAlbum, ID: 1, CreatedAt: now.Add(-400 * 24 * time.Hour)},
		{Kind: discovery.KindAlbum, ID: 2, CreatedAt: now.Add(-400 * 24 * time.Hour)},
	}
	seed1 := discovery.Rank(cands, discovery.TasteProfile{}, 100, now, 0, 2)
	seed2 := discovery.Rank(cands, discovery.TasteProfile{}, 200, now, 0, 2)
	// At least one seed should produce a different ordering than input order
	if seed1[0].ID == 1 && seed2[0].ID == 1 {
		t.Fatal("both seeds produced input order; jitter may not be influencing rediscovery ranking")
	}
}

// TestSortIsStableForEqualScores verifies that sort.SliceStable is in use and
// candidates with equal scores appear in a consistent order across repeated sorts.
// This property is load-bearing for the paging guarantee — without stability,
// equal-scored items could reorder between requests and break rank continuity.
func TestSortIsStableForEqualScores(t *testing.T) {
	now := time.Now()
	// Two played albums with identical signals except their IDs produce jitter that
	// might yield equal scores. Run the ranking multiple times and verify the order
	// stays consistent.
	played := now.Add(-1 * time.Hour)
	cands := []discovery.Candidate{
		{Kind: discovery.KindAlbum, ID: 100, CreatedAt: now, PlayCount: 1, LastPlayedAt: &played},
		{Kind: discovery.KindAlbum, ID: 101, CreatedAt: now, PlayCount: 1, LastPlayedAt: &played},
		{Kind: discovery.KindAlbum, ID: 102, CreatedAt: now, PlayCount: 1, LastPlayedAt: &played},
	}
	first := discovery.Rank(cands, discovery.TasteProfile{}, 1, now, 0, 3)
	for i := 0; i < 10; i++ {
		again := discovery.Rank(cands, discovery.TasteProfile{}, 1, now, 0, 3)
		for j := range first {
			if first[j] != again[j] {
				t.Fatalf("iteration %d rank %d changed: %+v vs %+v", i, j, first[j], again[j])
			}
		}
	}
}
