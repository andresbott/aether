package discovery_test

import (
	"math"
	"testing"
	"time"

	"github.com/andresbott/aether/internal/discovery"
)

const eps = 1e-9

func TestDecayAtZeroAgeIsOne(t *testing.T) {
	if got := discovery.Decay(0, discovery.TasteHalfLife); math.Abs(got-1) > eps {
		t.Fatalf("Decay(0) = %v, want 1", got)
	}
}

func TestDecayAtOneHalfLifeIsHalf(t *testing.T) {
	got := discovery.Decay(discovery.TasteHalfLife, discovery.TasteHalfLife)
	if math.Abs(got-0.5) > eps {
		t.Fatalf("Decay(halfLife) = %v, want 0.5", got)
	}
}

func TestDecayNeverNegativeForNegativeAge(t *testing.T) {
	// A clock skew could hand us a future timestamp; it must clamp to 1, not blow up.
	if got := discovery.Decay(-time.Hour, discovery.TasteHalfLife); got != 1 {
		t.Fatalf("Decay(negative) = %v, want 1", got)
	}
}

func TestAffinityOfEmptyProfileIsZero(t *testing.T) {
	p := discovery.BuildTasteProfile(nil, nil, time.Now())
	if got := p.Affinity([]uint{1, 2}); got != 0 {
		t.Fatalf("empty profile Affinity = %v, want 0", got)
	}
}

func TestProfileIsL1Normalised(t *testing.T) {
	now := time.Now()
	plays := []discovery.GenrePlay{
		{GenreID: 1, PlayedAt: now},
		{GenreID: 1, PlayedAt: now},
		{GenreID: 2, PlayedAt: now},
	}
	p := discovery.BuildTasteProfile(plays, nil, now)
	sum := p.Affinity([]uint{1}) + p.Affinity([]uint{2})
	if math.Abs(sum-1) > eps {
		t.Fatalf("weights sum to %v, want 1", sum)
	}
	if p.Affinity([]uint{1}) <= p.Affinity([]uint{2}) {
		t.Fatal("genre 1 has twice the plays and must outweigh genre 2")
	}
}

func TestRecentPlayOutweighsOldPlay(t *testing.T) {
	now := time.Now()
	plays := []discovery.GenrePlay{
		{GenreID: 1, PlayedAt: now},
		{GenreID: 2, PlayedAt: now.Add(-discovery.TasteHalfLife)},
	}
	p := discovery.BuildTasteProfile(plays, nil, now)
	if p.Affinity([]uint{1}) <= p.Affinity([]uint{2}) {
		t.Fatal("a fresh play must outweigh one a half-life old")
	}
}

// The horizon is what keeps the SQL bounded; the pure code must agree with it so
// a row the store failed to filter cannot sneak weight in.
func TestPlaysPastHorizonAreIgnored(t *testing.T) {
	now := time.Now()
	plays := []discovery.GenrePlay{
		{GenreID: 1, PlayedAt: now},
		{GenreID: 2, PlayedAt: now.Add(-discovery.TasteHorizon - time.Hour)},
	}
	p := discovery.BuildTasteProfile(plays, nil, now)
	if p.Affinity([]uint{2}) != 0 {
		t.Fatalf("play past the horizon contributed %v, want 0", p.Affinity([]uint{2}))
	}
	if math.Abs(p.Affinity([]uint{1})-1) > eps {
		t.Fatalf("surviving genre should hold all the weight, got %v", p.Affinity([]uint{1}))
	}
}

func TestStarsCountAtFullWeightRegardlessOfAge(t *testing.T) {
	now := time.Now()
	// One very old play vs one star: the star must win, since stars ignore age.
	plays := []discovery.GenrePlay{{GenreID: 1, PlayedAt: now.Add(-500 * 24 * time.Hour)}}
	stars := []discovery.GenreStar{{GenreID: 2}}
	p := discovery.BuildTasteProfile(plays, stars, now)
	if p.Affinity([]uint{2}) <= p.Affinity([]uint{1}) {
		t.Fatal("a star must outweigh a 500-day-old play")
	}
}

func TestAffinityAveragesOverGenres(t *testing.T) {
	now := time.Now()
	plays := []discovery.GenrePlay{
		{GenreID: 1, PlayedAt: now},
		{GenreID: 1, PlayedAt: now},
		{GenreID: 1, PlayedAt: now},
		{GenreID: 2, PlayedAt: now},
	}
	p := discovery.BuildTasteProfile(plays, nil, now)
	both := p.Affinity([]uint{1, 2})
	mean := (p.Affinity([]uint{1}) + p.Affinity([]uint{2})) / 2
	if math.Abs(both-mean) > eps {
		t.Fatalf("Affinity over two genres = %v, want their mean %v", both, mean)
	}
}

func TestAffinityOfUnknownGenreIsZero(t *testing.T) {
	now := time.Now()
	p := discovery.BuildTasteProfile([]discovery.GenrePlay{{GenreID: 1, PlayedAt: now}}, nil, now)
	if got := p.Affinity([]uint{999}); got != 0 {
		t.Fatalf("unknown genre Affinity = %v, want 0", got)
	}
}

func TestAffinityOfNoGenresIsZero(t *testing.T) {
	now := time.Now()
	p := discovery.BuildTasteProfile([]discovery.GenrePlay{{GenreID: 1, PlayedAt: now}}, nil, now)
	if got := p.Affinity(nil); got != 0 {
		t.Fatalf("Affinity(nil) = %v, want 0", got)
	}
}

// A play at exactly the horizon is retained, not dropped: the store's SQL
// predicate is `played_at >= cutoff`, so the pure code must agree at the
// boundary or the two disagree about one edge play.
func TestPlayExactlyAtHorizonIsRetained(t *testing.T) {
	now := time.Now()
	plays := []discovery.GenrePlay{{GenreID: 1, PlayedAt: now.Add(-discovery.TasteHorizon)}}
	p := discovery.BuildTasteProfile(plays, nil, now)
	if got := p.Affinity([]uint{1}); got == 0 {
		t.Fatal("play at exactly TasteHorizon was dropped; SQL uses >= cutoff so it must be kept")
	}
}

// Later tasks pass a zero-value TasteProfile deliberately (e.g. when the taste
// query failed and the feed degrades to no genre signal). A nil map must read as
// zero affinity, not panic.
func TestAffinityOnZeroValueProfileIsZero(t *testing.T) {
	var p discovery.TasteProfile
	if got := p.Affinity([]uint{1, 2, 3}); got != 0 {
		t.Fatalf("zero-value profile Affinity = %v, want 0", got)
	}
}

// A star must count as exactly one full-weight unit, not merely "more than an old
// play". With one star and one fresh play on different genres, each holds half the
// normalised weight — which only holds if a star weighs exactly 1.
func TestStarWeighsExactlyOneUnit(t *testing.T) {
	now := time.Now()
	plays := []discovery.GenrePlay{{GenreID: 1, PlayedAt: now}}
	stars := []discovery.GenreStar{{GenreID: 2}}
	p := discovery.BuildTasteProfile(plays, stars, now)
	if math.Abs(p.Affinity([]uint{1})-0.5) > eps {
		t.Fatalf("fresh play weight = %v, want 0.5", p.Affinity([]uint{1}))
	}
	if math.Abs(p.Affinity([]uint{2})-0.5) > eps {
		t.Fatalf("star weight = %v, want 0.5", p.Affinity([]uint{2}))
	}
}
