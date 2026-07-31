package discovery

import (
	"math"
	"time"
)

// Per-term tuning. Added-recency decays faster than taste (30d vs 90d) because
// "new to the library" is a shorter-lived claim than "a genre you like", and
// play-recency faster still (14d) so yesterday's rotation fades within a
// fortnight instead of pinning the feed.
const (
	AddedRecencyHalfLife = 30 * 24 * time.Hour
	PlayRecencyHalfLife  = 14 * 24 * time.Hour

	// FamiliaritySaturation is the play count at which PlayFamiliarity reaches 1.
	// Past it the term clamps, so a 200-play album barely outranks a 40-play one
	// and long-standing favorites cannot monopolise the feed.
	FamiliaritySaturation = 64
)

// Score weights. They sum to 1, so a Score is itself in [0,1] and each weight
// reads as that signal's share of the ranking.
const (
	WeightAddedRecency    = 0.20
	WeightFavorite        = 0.25
	WeightPlayFamiliarity = 0.20
	WeightPlayRecency     = 0.15
	WeightGenreAffinity   = 0.15
	WeightJitter          = 0.05
)

// Kind distinguishes the two entity types that compete in one ranking. The
// values double as the jitter hash prefix and the store's id namespace.
type Kind string

const (
	KindAlbum    Kind = "al"
	KindPlaylist Kind = "pl"
)

// Reason is the badge a feed item shows to explain why it surfaced. With the
// five themed shelves gone, this is the only thing left telling the user that.
type Reason string

const (
	ReasonFavorite       Reason = "favorite"
	ReasonRecentlyAdded  Reason = "recentlyAdded"
	ReasonMostPlayed     Reason = "mostPlayed"
	ReasonRecentlyPlayed Reason = "recentlyPlayed"
	ReasonGenreMatch     Reason = "genreMatch"
	ReasonRediscover     Reason = "rediscover"
)

// Candidate is one scorable item with its raw signals already gathered. Pointer
// timestamps are nil for "never happened", matching the store's map-absence
// contract for stars and play stats.
type Candidate struct {
	Kind         Kind
	ID           uint
	CreatedAt    time.Time
	StarredAt    *time.Time
	PlayCount    int
	LastPlayedAt *time.Time
	GenreIDs     []uint
}

// Terms holds one candidate's six normalised signals, each in [0,1].
type Terms struct {
	AddedRecency    float64
	Favorite        float64
	PlayFamiliarity float64
	PlayRecency     float64
	GenreAffinity   float64
	Jitter          float64
}

// NeverPlayed reports whether a candidate belongs in the rediscovery pool. It
// keys on PlayCount rather than LastPlayedAt so a play row with an unparseable
// timestamp still counts as played.
func NeverPlayed(c Candidate) bool {
	return c.PlayCount <= 0
}

// ComputeTerms normalises every raw signal on a candidate to [0,1].
func ComputeTerms(c Candidate, p TasteProfile, seed int64, now time.Time) Terms {
	t := Terms{
		AddedRecency:  Decay(now.Sub(c.CreatedAt), AddedRecencyHalfLife),
		GenreAffinity: p.Affinity(c.GenreIDs),
		Jitter:        Jitter(seed, string(c.Kind), c.ID),
	}
	if c.StarredAt != nil {
		t.Favorite = 1
	}
	if c.PlayCount > 0 {
		// log1p keeps the curve steep where it matters (0 -> 10 plays) and flat
		// where it does not (100 -> 200).
		t.PlayFamiliarity = math.Min(
			1,
			math.Log1p(float64(c.PlayCount))/math.Log1p(FamiliaritySaturation),
		)
	}
	if c.LastPlayedAt != nil {
		t.PlayRecency = Decay(now.Sub(*c.LastPlayedAt), PlayRecencyHalfLife)
	}
	return t
}

// Score is the weighted sum. Both kinds use the same weights, which is what
// makes cross-type interleaving honest: the numbers are directly comparable.
func (t Terms) Score() float64 {
	return WeightAddedRecency*t.AddedRecency +
		WeightFavorite*t.Favorite +
		WeightPlayFamiliarity*t.PlayFamiliarity +
		WeightPlayRecency*t.PlayRecency +
		WeightGenreAffinity*t.GenreAffinity +
		WeightJitter*t.Jitter
}

// Reason names the term contributing the most weighted score, skipping jitter —
// "we rolled a die" is not an explanation worth showing. When nothing else
// contributes, it falls back to genreMatch, the most neutral claim available.
func (t Terms) Reason() Reason {
	contributions := []struct {
		weighted float64
		reason   Reason
	}{
		{WeightFavorite * t.Favorite, ReasonFavorite},
		{WeightAddedRecency * t.AddedRecency, ReasonRecentlyAdded},
		{WeightPlayFamiliarity * t.PlayFamiliarity, ReasonMostPlayed},
		{WeightPlayRecency * t.PlayRecency, ReasonRecentlyPlayed},
		{WeightGenreAffinity * t.GenreAffinity, ReasonGenreMatch},
	}
	best := ReasonGenreMatch
	var bestWeighted float64
	for _, c := range contributions {
		if c.weighted > bestWeighted {
			bestWeighted, best = c.weighted, c.reason
		}
	}
	return best
}
