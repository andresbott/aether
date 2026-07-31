package discovery_test

import (
	"math"
	"testing"
	"time"

	"github.com/andresbott/aether/internal/discovery"
)

func ptrTime(t time.Time) *time.Time { return &t }

func TestFreshAlbumScoresFullAddedRecency(t *testing.T) {
	now := time.Now()
	c := discovery.Candidate{Kind: discovery.KindAlbum, ID: 1, CreatedAt: now}
	terms := discovery.ComputeTerms(c, discovery.TasteProfile{}, 1, now)
	if math.Abs(terms.AddedRecency-1) > eps {
		t.Fatalf("AddedRecency = %v, want 1", terms.AddedRecency)
	}
}

func TestAddedRecencyHalvesAtItsHalfLife(t *testing.T) {
	now := time.Now()
	c := discovery.Candidate{
		Kind:      discovery.KindAlbum,
		ID:        1,
		CreatedAt: now.Add(-discovery.AddedRecencyHalfLife),
	}
	terms := discovery.ComputeTerms(c, discovery.TasteProfile{}, 1, now)
	if math.Abs(terms.AddedRecency-0.5) > eps {
		t.Fatalf("AddedRecency = %v, want 0.5", terms.AddedRecency)
	}
}

func TestFavoriteIsOneWhenStarredAndZeroOtherwise(t *testing.T) {
	now := time.Now()
	starred := discovery.Candidate{
		Kind: discovery.KindAlbum, ID: 1, CreatedAt: now, StarredAt: ptrTime(now),
	}
	plain := discovery.Candidate{Kind: discovery.KindAlbum, ID: 2, CreatedAt: now}
	if got := discovery.ComputeTerms(starred, discovery.TasteProfile{}, 1, now).Favorite; got != 1 {
		t.Fatalf("starred Favorite = %v, want 1", got)
	}
	if got := discovery.ComputeTerms(plain, discovery.TasteProfile{}, 1, now).Favorite; got != 0 {
		t.Fatalf("unstarred Favorite = %v, want 0", got)
	}
}

func TestPlayFamiliaritySaturates(t *testing.T) {
	now := time.Now()
	terms := func(plays int) discovery.Terms {
		c := discovery.Candidate{
			Kind: discovery.KindAlbum, ID: 1, CreatedAt: now, PlayCount: plays,
		}
		return discovery.ComputeTerms(c, discovery.TasteProfile{}, 1, now)
	}
	zero := terms(0).PlayFamiliarity
	forty := terms(40).PlayFamiliarity
	twoHundred := terms(200).PlayFamiliarity

	if zero != 0 {
		t.Fatalf("0 plays gave %v, want 0", zero)
	}
	if !(forty > 0) {
		t.Fatal("40 plays must score above zero")
	}
	if twoHundred > 1 {
		t.Fatalf("PlayFamiliarity = %v, must be clamped to 1", twoHundred)
	}
	// The point of the log curve: heavy rotation must not dwarf moderate rotation.
	if twoHundred-forty > 0.35 {
		t.Fatalf("200 plays beat 40 by %v; the curve is not saturating", twoHundred-forty)
	}
}

func TestPlayRecencyIsZeroWhenNeverPlayed(t *testing.T) {
	now := time.Now()
	c := discovery.Candidate{Kind: discovery.KindAlbum, ID: 1, CreatedAt: now}
	if got := discovery.ComputeTerms(c, discovery.TasteProfile{}, 1, now).PlayRecency; got != 0 {
		t.Fatalf("PlayRecency = %v, want 0", got)
	}
}

func TestPlayRecencyHalvesAtItsHalfLife(t *testing.T) {
	now := time.Now()
	c := discovery.Candidate{
		Kind:         discovery.KindAlbum,
		ID:           1,
		CreatedAt:    now,
		PlayCount:    1,
		LastPlayedAt: ptrTime(now.Add(-discovery.PlayRecencyHalfLife)),
	}
	got := discovery.ComputeTerms(c, discovery.TasteProfile{}, 1, now).PlayRecency
	if math.Abs(got-0.5) > eps {
		t.Fatalf("PlayRecency = %v, want 0.5", got)
	}
}

func TestGenreAffinityComesFromTheProfile(t *testing.T) {
	now := time.Now()
	profile := discovery.BuildTasteProfile(
		[]discovery.GenrePlay{{GenreID: 7, PlayedAt: now}}, nil, now,
	)
	match := discovery.Candidate{
		Kind: discovery.KindAlbum, ID: 1, CreatedAt: now, GenreIDs: []uint{7},
	}
	miss := discovery.Candidate{
		Kind: discovery.KindAlbum, ID: 2, CreatedAt: now, GenreIDs: []uint{8},
	}
	if got := discovery.ComputeTerms(match, profile, 1, now).GenreAffinity; math.Abs(got-1) > eps {
		t.Fatalf("matching GenreAffinity = %v, want 1", got)
	}
	if got := discovery.ComputeTerms(miss, profile, 1, now).GenreAffinity; got != 0 {
		t.Fatalf("non-matching GenreAffinity = %v, want 0", got)
	}
}

func TestScoreIsTheWeightedSum(t *testing.T) {
	terms := discovery.Terms{
		AddedRecency:    1,
		Favorite:        1,
		PlayFamiliarity: 1,
		PlayRecency:     1,
		GenreAffinity:   1,
		Jitter:          1,
	}
	// Every weight at full strength must sum to exactly 1.
	if got := terms.Score(); math.Abs(got-1) > eps {
		t.Fatalf("all-ones Score = %v, want 1", got)
	}
	if got := (discovery.Terms{}).Score(); got != 0 {
		t.Fatalf("all-zeros Score = %v, want 0", got)
	}
}

// Every weight's exact value, not just their sum and not just a partial ordering.
// A compensating swap (e.g. familiarity 0.20 <-> recency 0.15) keeps the sum at 1.0
// and would otherwise pass the suite while silently inverting the spec's intent:
// "played often" must outweigh "played recently".
func TestScoreWeightsAreExact(t *testing.T) {
	cases := []struct {
		name  string
		terms discovery.Terms
		want  float64
	}{
		{"favorite", discovery.Terms{Favorite: 1}, 0.25},
		{"addedRecency", discovery.Terms{AddedRecency: 1}, 0.20},
		{"playFamiliarity", discovery.Terms{PlayFamiliarity: 1}, 0.20},
		{"playRecency", discovery.Terms{PlayRecency: 1}, 0.15},
		{"genreAffinity", discovery.Terms{GenreAffinity: 1}, 0.15},
		{"jitter", discovery.Terms{Jitter: 1}, 0.05},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.terms.Score(); math.Abs(got-tc.want) > eps {
				t.Fatalf("%s weight = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

// The ordering the spec depends on, stated as relations rather than values so the
// intent survives a future retune: favorites lead, familiarity ties added-recency
// and must not fall below play-recency, and jitter is always the smallest.
func TestScoreWeightOrdering(t *testing.T) {
	fav := discovery.Terms{Favorite: 1}.Score()
	added := discovery.Terms{AddedRecency: 1}.Score()
	familiarity := discovery.Terms{PlayFamiliarity: 1}.Score()
	recency := discovery.Terms{PlayRecency: 1}.Score()
	genre := discovery.Terms{GenreAffinity: 1}.Score()
	jitter := discovery.Terms{Jitter: 1}.Score()

	if fav <= added {
		t.Fatalf("favorite (%v) must outweigh addedRecency (%v)", fav, added)
	}
	if math.Abs(familiarity-added) > eps {
		t.Fatalf("playFamiliarity (%v) must equal addedRecency (%v)", familiarity, added)
	}
	if familiarity <= recency {
		t.Fatalf("playFamiliarity (%v) must outweigh playRecency (%v) — "+
			"played often beats played recently", familiarity, recency)
	}
	if math.Abs(recency-genre) > eps {
		t.Fatalf("playRecency (%v) must equal genreAffinity (%v)", recency, genre)
	}
	if jitter >= genre {
		t.Fatalf("jitter (%v) must be the smallest weight, below genre (%v)", jitter, genre)
	}
}

func TestReasonPicksTheHighestWeightedContributor(t *testing.T) {
	cases := []struct {
		name  string
		terms discovery.Terms
		want  discovery.Reason
	}{
		{"starred", discovery.Terms{Favorite: 1}, discovery.ReasonFavorite},
		{"fresh", discovery.Terms{AddedRecency: 1}, discovery.ReasonRecentlyAdded},
		{"heavy rotation", discovery.Terms{PlayFamiliarity: 1}, discovery.ReasonMostPlayed},
		{"just played", discovery.Terms{PlayRecency: 1}, discovery.ReasonRecentlyPlayed},
		{"genre match", discovery.Terms{GenreAffinity: 1}, discovery.ReasonGenreMatch},
		// Favorite (0.25) beats AddedRecency (0.20) when both are saturated.
		{"starred and fresh", discovery.Terms{Favorite: 1, AddedRecency: 1}, discovery.ReasonFavorite},
		// A weak favorite loses to a strong added-recency contribution:
		// 0.25*0.1 = 0.025 vs 0.20*1 = 0.20.
		{"weak star, fresh", discovery.Terms{Favorite: 0.1, AddedRecency: 1}, discovery.ReasonRecentlyAdded},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.terms.Reason(); got != tc.want {
				t.Fatalf("Reason = %q, want %q", got, tc.want)
			}
		})
	}
}

// Jitter must never be the stated reason — it is a tiebreaker, not an
// explanation the user would accept.
func TestReasonNeverReportsJitter(t *testing.T) {
	if got := (discovery.Terms{Jitter: 1}).Reason(); got == discovery.Reason("jitter") {
		t.Fatal("jitter surfaced as a reason")
	}
	if got := (discovery.Terms{Jitter: 1}).Reason(); got != discovery.ReasonGenreMatch {
		t.Fatalf("all-jitter Reason = %q, want the zero-contribution fallback %q",
			got, discovery.ReasonGenreMatch)
	}
}

func TestNeverPlayed(t *testing.T) {
	now := time.Now()
	unplayed := discovery.Candidate{Kind: discovery.KindAlbum, ID: 1, CreatedAt: now}
	played := discovery.Candidate{
		Kind: discovery.KindAlbum, ID: 2, CreatedAt: now, PlayCount: 3,
		LastPlayedAt: ptrTime(now),
	}
	if !discovery.NeverPlayed(unplayed) {
		t.Fatal("a zero-play candidate must be NeverPlayed")
	}
	if discovery.NeverPlayed(played) {
		t.Fatal("a played candidate must not be NeverPlayed")
	}
}

func TestComputeTermsFillsJitterFromTheSeed(t *testing.T) {
	now := time.Now()
	c := discovery.Candidate{Kind: discovery.KindAlbum, ID: 5, CreatedAt: now}
	got := discovery.ComputeTerms(c, discovery.TasteProfile{}, 99, now).Jitter
	want := discovery.Jitter(99, string(discovery.KindAlbum), 5)
	if got != want {
		t.Fatalf("Jitter term = %v, want %v", got, want)
	}
}

// Pins FamiliaritySaturation to exactly 64 plays. The brief's saturates test
// uses inequalities; this catches mutations like 64 -> 32 by checking that
// at 32 plays we're significantly below the saturation (< 0.9).
func TestFamiliaritySaturationIsExactly64(t *testing.T) {
	now := time.Now()
	at32 := discovery.ComputeTerms(
		discovery.Candidate{Kind: discovery.KindAlbum, ID: 1, CreatedAt: now, PlayCount: 32},
		discovery.TasteProfile{}, 1, now,
	).PlayFamiliarity
	// With saturation at 64, PlayFamiliarity at 32 plays should be well below 0.9
	// (closer to 0.82). But with saturation at 32, we'd get 1.0. This catches that.
	if at32 > 0.90 {
		t.Fatalf("PlayFamiliarity at 32 plays = %v, should be < 0.90; saturation may not be 64", at32)
	}
	if at32 < 0.75 {
		t.Fatalf("PlayFamiliarity at 32 plays = %v, should be > 0.75; saturation may not be 64", at32)
	}
}

// Pins PlayRecencyHalfLife to exactly 14 days. The brief's play-recency test
// uses the constant directly; this catches mutations like 14d -> 21d.
func TestPlayRecencyHalfLifeIsExactly14Days(t *testing.T) {
	now := time.Now()
	twoWeeksAgo := now.Add(-14 * 24 * time.Hour)
	c := discovery.Candidate{
		Kind:         discovery.KindAlbum,
		ID:           1,
		CreatedAt:    now,
		PlayCount:    1,
		LastPlayedAt: ptrTime(twoWeeksAgo),
	}
	got := discovery.ComputeTerms(c, discovery.TasteProfile{}, 1, now).PlayRecency
	// At exactly 14 days, should be 0.5 (half of max).
	if math.Abs(got-0.5) > eps {
		t.Fatalf("PlayRecency at 14d = %v, want 0.5", got)
	}
}
