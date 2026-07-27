package albumidentify

import (
	"sort"

	"github.com/andresbott/aether/libs/acoustid"
)

// unionReleases folds the per-file fingerprint results into one option per
// release MBID any candidate mentioned. A file contributes at most one
// assignment per release — its highest-scoring candidate there — so an album
// whose tracklist repeats a recording cannot inflate its own coverage.
//
// Files that failed or matched nothing contribute no assignment: they are
// carried by the caller and either gap-filled (fill.go) or reported as
// unmatched.
func unionReleases(results []fileResult) []*AlbumOption {
	byMBID := make(map[string]*AlbumOption)
	// Insertion order of first appearance, so the output is deterministic
	// before ranking sorts it.
	var order []string
	for _, res := range results {
		if res.err != nil {
			continue
		}
		best := bestPerRelease(res.recordings)
		// Sort the release MBIDs to ensure deterministic order when a file
		// matches multiple releases.
		mbids := make([]string, 0, len(best))
		for mbid := range best {
			mbids = append(mbids, mbid)
		}
		sort.Strings(mbids)
		for _, mbid := range mbids {
			m := best[mbid]
			opt, ok := byMBID[mbid]
			if !ok {
				opt = &AlbumOption{
					ReleaseMBID:      mbid,
					ReleaseGroupMBID: m.release.ReleaseGroupMBID,
					Album:            m.release.Title,
					Year:             m.release.Year,
				}
				byMBID[mbid] = opt
				order = append(order, mbid)
			}
			opt.Assignments = append(opt.Assignments, Assignment{
				Path:          res.input.Path,
				Source:        SourceFingerprint,
				Title:         m.recording.Title,
				RecordingMBID: m.recording.MBID,
				Artists:       toArtists(m.recording.Artists),
				DiscNumber:    m.release.DiscNumber,
				TrackNumber:   m.release.TrackNumber,
				Score:         m.recording.Score,
			})
		}
	}
	out := make([]*AlbumOption, 0, len(order))
	for _, mbid := range order {
		opt := byMBID[mbid]
		// Assignments arrive in input order per release; keep them so, and
		// summarise coverage.
		sort.SliceStable(opt.Assignments, func(i, j int) bool {
			return opt.Assignments[i].TrackNumber < opt.Assignments[j].TrackNumber
		})
		opt.MatchedCount = len(opt.Assignments)
		var sum float64
		for _, a := range opt.Assignments {
			sum += a.Score
		}
		if opt.MatchedCount > 0 {
			opt.MeanScore = sum / float64(opt.MatchedCount)
		}
		out = append(out, opt)
	}
	return out
}

// releaseMatch is one file's best candidate on one release.
type releaseMatch struct {
	recording acoustid.Recording
	release   acoustid.Release
}

// bestPerRelease reduces one file's candidate list to its highest-scoring
// candidate per release MBID. Releases without an MBID are unusable as a tag
// target and are dropped.
func bestPerRelease(recs []acoustid.Recording) map[string]releaseMatch {
	out := make(map[string]releaseMatch)
	for _, r := range recs {
		for _, rel := range r.Release {
			if rel.MBID == "" {
				continue
			}
			if cur, ok := out[rel.MBID]; ok && cur.recording.Score >= r.Score {
				continue
			}
			out[rel.MBID] = releaseMatch{recording: r, release: rel}
		}
	}
	return out
}

func toArtists(credits []acoustid.ArtistCredit) []Artist {
	if len(credits) == 0 {
		return nil
	}
	out := make([]Artist, 0, len(credits))
	for _, c := range credits {
		out = append(out, Artist{Name: c.Name, MBID: c.MBID})
	}
	return out
}
