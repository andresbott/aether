#!/usr/bin/env bash
# Download a small library of freely licensed albums from the Internet Archive
# into <dest> (`make sample-library` passes zarf/locallibrary/sample-library,
# which `make run` scans). About 1 GB; everything is CC-licensed or shared on
# the Live Music Archive with the artist's permission — fine for local
# development, not for committing or redistributing.
#
# Re-runs are cheap: files already on disk with the right md5 are skipped, so
# an interrupted download resumes where it stopped. Most items are served by a
# single archive.org node, which is sometimes down: an album that fails is
# skipped, the rest still download, and the run ends non-zero listing what is
# missing — re-run later to fill the gaps. Requires curl and jq.
set -euo pipefail

dest="${1:?usage: fetch.sh <dest-dir>}"

for tool in curl jq; do
	command -v "$tool" >/dev/null || { echo "$tool is required" >&2; exit 1; }
done

# Each album is chosen for something the scanner or the UI has to handle.
# Fields: target dir | archive.org item | file format to take (originals only) |
# item file saved as cover.jpg ("-" for none) | license
albums=(
	# Tagged 320 kbps MP3s with a folder cover.jpg only; same artist as The Slip.
	"Nine Inch Nails/2008 - Ghosts I-IV|nineinchnails_ghosts_I_IV|320Kbps MP3|cover.jpg|CC BY-NC-SA 3.0 US"
	# FLAC with embedded 1200px art plus a folder cover, composer and BPM tags.
	"Nine Inch Nails/2008 - The Slip|nine_inch_nails_the_slip|Flac|cover.jpg|CC BY-NC-SA 3.0 US"
	# Classical: performer as artist, J.S. Bach as composer, ISRCs, 32 tracks.
	"Kimiko Ishizaka/2012 - The Open Goldberg Variations|The_Open_Goldberg_Variations-11823|VBR MP3|The_Open_Goldberg_Variations-11823.jpg|CC0 1.0"
	# Two short film scores by one composer: genre Soundtrack, "n/total" track
	# numbers, and non-square covers (2:1 front+back spreads).
	"Jan Morgenstern/2006 - Elephants Dream|JanMorgenstern-ElephantsDream|VBR MP3|cover.jpg|CC BY-NC-ND 2.5"
	"Jan Morgenstern/2008 - Big Buck Bunny|JanMorgenstern-BigBuckBunny|VBR MP3|Cover-150dpi.jpg|CC BY-NC-ND 3.0"
	# One artist across two genres (Funk, Jazz); tiny 121px covers.
	"Kevin MacLeod/2011 - Funk Sampler|Funk_Sampler-9613|VBR MP3|Funk_Sampler-9613.jpg|CC BY 3.0"
	"Kevin MacLeod/2011 - Jazz Sampler|Jazz_Sampler-9619|VBR MP3|Jazz_Sampler-9619.jpg|CC BY 3.0"
	# Various-artists compilation with accented names; embedded art only.
	"Various Artists/2012 - July|July-12126|VBR MP3|-|CC BY-NC-SA 3.0 / CC BY-ND 3.0 (per track)"
	# Live soundboard FLACs with no tags but ReplayGain, and no art at all.
	"Elliott Smith/1997-05-06 - KCRW Morning Becomes Eclectic|esmith1997-05-06MBE|Flac|-|Live Music Archive (non-commercial trading)"
)

md5_of() {
	if command -v md5sum >/dev/null; then md5sum "$1" | cut -d' ' -f1; else md5 -q "$1"; fi
}

curl_opts=(-fsSL --retry 5 --retry-delay 5 --retry-connrefused --connect-timeout 20)
failed=()

fetch_file() { # fetch_file <url> <md5> <target>
	if [ -f "$3" ] && [ "$(md5_of "$3")" = "$2" ]; then
		return 0
	fi
	mkdir -p "$(dirname "$3")"
	echo "  ${3#"$dest"/}"
	if ! curl "${curl_opts[@]}" -o "$3.part" "$1"; then
		rm -f "$3.part"
		return 1
	fi
	if [ "$(md5_of "$3.part")" != "$2" ]; then
		rm -f "$3.part"
		echo "  md5 mismatch: $1" >&2
		return 1
	fi
	mv "$3.part" "$3"
}

fetch_album() { # fetch_album <dir> <item> <format> <cover>
	local meta files url md5 name
	echo "$1"
	if ! meta="$(curl "${curl_opts[@]}" "https://archive.org/metadata/$2")" ||
		! files="$(jq -er --arg id "$2" --arg f "$3" --arg c "$4" '
			[ .files[]
			  | select((.format == $f and .source == "original") or .name == $c)
			  | [ "https://archive.org/download/\($id)/\(.name | split("/") | map(@uri) | join("/"))",
			      .md5,
			      (if .name == $c then "cover.jpg" else .name end) ]
			  | @tsv ]
			| if length > 0 then join("\n") else error("no \($f) files in \($id)") end' <<<"$meta")"; then
		failed+=("$1")
		return 0
	fi
	# An album's files share one node: after a failure the rest would only
	# burn through the same retries, so move on to the next album.
	while IFS=$'\t' read -r url md5 name; do
		if ! fetch_file "$url" "$md5" "$dest/$1/$name"; then
			failed+=("$1")
			return 0
		fi
	done <<<"$files"
}

mkdir -p "$dest"
{
	echo "# Sample library"
	echo
	echo "Fetched by \`make sample-library\` from the Internet Archive for local"
	echo "development. Do not commit or redistribute."
	echo
	echo "| Album | License | Source |"
	echo "|---|---|---|"
	for entry in "${albums[@]}"; do
		IFS='|' read -r dir item _ _ license <<<"$entry"
		echo "| $dir | $license | https://archive.org/details/$item |"
	done
} >"$dest/CREDITS.md"

for entry in "${albums[@]}"; do
	IFS='|' read -r dir item format cover _ <<<"$entry"
	fetch_album "$dir" "$item" "$format" "$cover"
done

if [ ${#failed[@]} -gt 0 ]; then
	echo "❌ incomplete, archive.org did not deliver:" >&2
	printf '   %s\n' "${failed[@]}" >&2
	echo "   re-run 'make sample-library' later; finished files are kept" >&2
	exit 1
fi
echo "✅ sample library ready in '$dest' (licenses in CREDITS.md)"
