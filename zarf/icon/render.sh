#!/usr/bin/env bash
# Render the SPA icon set from the cleaned web sources in zarf/icon/web into
# webui/public, where Vite copies them verbatim into webui/dist and `make
# package-ui` embeds them in the Go binary.
#
# The outputs are committed, so this only needs re-running when the artwork
# changes (`make icons` from the repo root). Requires inkscape (SVG -> PNG) and
# ImageMagick (PNG -> ICO).
set -euo pipefail

src="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/web"
out="$(cd "$src/../../.." && pwd)/webui/public"

for tool in inkscape convert; do
	command -v "$tool" >/dev/null || { echo "$tool is required" >&2; exit 1; }
done

render() { # render <source.svg> <size> <target.png>
	inkscape -w "$2" -h "$2" -o "$out/$3" "$src/$1" >/dev/null 2>&1
	echo "  $3 (${2}x${2})"
}

echo "rendering icons into webui/public"

# Scalable favicon: modern browsers prefer it over any raster in the tab strip,
# and the manifest lists it as the resolution-independent "any" icon.
cp "$src/icon.svg" "$out/icon.svg"
echo "  icon.svg"

# PWA / Android install icons. 192 is the launcher icon, 512 the splash source.
render icon.svg 192 icon-192.png
render icon.svg 512 icon-512.png

# Adaptive-icon art with the mark inside the 80% safe zone.
render icon-maskable.svg 192 icon-maskable-192.png
render icon-maskable.svg 512 icon-maskable-512.png

# iOS home screen. 180 is the largest size Safari asks for (iPhone @3x); it
# downsamples for iPad and older devices. Flattened: no alpha, square corners.
inkscape -w 180 -h 180 -o "$out/apple-touch-icon.png" "$src/icon-square.svg" >/dev/null 2>&1
convert "$out/apple-touch-icon.png" -background '#102744' -alpha remove -alpha off "$out/apple-touch-icon.png"
echo "  apple-touch-icon.png (180x180)"

# favicon.ico still matters: bookmark bars, pinned Windows shortcuts and any
# client that asks for /favicon.ico without reading the HTML.
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
for size in 16 32 48; do
	inkscape -w "$size" -h "$size" -o "$tmp/$size.png" "$src/icon.svg" >/dev/null 2>&1
done
convert "$tmp/16.png" "$tmp/32.png" "$tmp/48.png" "$out/favicon.ico"
echo "  favicon.ico (16, 32, 48)"
