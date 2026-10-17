#!/usr/bin/env bash
#
# Rasterise the authored branding sources in this directory into the web assets
# the Hugo site serves from site/static/.
#
# The outputs are committed, so this only needs re-running when a source .svg
# here changes -- CI never invokes it and therefore never needs a rasteriser
# installed. Run it via `make generate-branding` from the repository root.
#
# Requires: python3 (for assembling the multi-size .ico), plus one of
# rsvg-convert, magick/convert, or npx (any of which can rasterise SVG).

set -euo pipefail

cd "$(dirname "$0")"
out="../site/static"
mkdir -p "$out/images"

# ---------------------------------------------------------------------------
# Pick a rasteriser. render <src.svg> <width> <height> <dest.png>
# ---------------------------------------------------------------------------
if command -v rsvg-convert >/dev/null 2>&1; then
  renderer="rsvg-convert"
  render() { rsvg-convert -w "$2" -h "$3" -o "$4" "$1"; }
elif command -v magick >/dev/null 2>&1; then
  renderer="magick"
  render() { magick -background none -density 384 "$1" -resize "${2}x${3}" "$4"; }
elif command -v convert >/dev/null 2>&1; then
  renderer="convert"
  render() { convert -background none -density 384 "$1" -resize "${2}x${3}" "$4"; }
elif command -v npx >/dev/null 2>&1; then
  # Downloads @resvg/resvg-js-cli on first use, so this branch needs network.
  renderer="npx @resvg/resvg-js-cli"
  render() { npx -y @resvg/resvg-js-cli --fit-width "$2" --fit-height "$3" "$1" "$4" >/dev/null; }
else
  cat >&2 <<'EOF'
branding/generate.sh: no SVG rasteriser found.

Install one of:
  brew install librsvg        # provides rsvg-convert (recommended, offline)
  brew install imagemagick    # provides magick
  node + npx                  # falls back to @resvg/resvg-js-cli (needs network)
EOF
  exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "branding/generate.sh: python3 is required to assemble favicon.ico" >&2
  exit 1
fi

echo "==> rasterising with $renderer"

# ---------------------------------------------------------------------------
# Vector assets: copied verbatim, never rasterised.
#
# favicon.svg is a separate source from mark.svg because it carries its own
# prefers-color-scheme media query (a favicon cannot see the page's stylesheet,
# so the site's dark-mode class toggle cannot reach it). The navbar logo, by
# contrast, is swapped by Hextra as a light/dark <img> pair, so it ships as two
# fixed-colour files instead.
# ---------------------------------------------------------------------------
cp favicon.svg   "$out/favicon.svg"
cp mark.svg      "$out/images/logo.svg"
cp mark-dark.svg "$out/images/logo-dark.svg"

# ---------------------------------------------------------------------------
# Raster assets. Every path below is one the Hextra theme's favicons.html
# already references; project files under site/static/ take precedence over the
# theme's own, which is what displaces Hextra's default branding.
# ---------------------------------------------------------------------------
render mark.svg          16   16   "$out/favicon-16x16.png"
render mark.svg          32   32   "$out/favicon-32x32.png"
render icon-opaque.svg   180  180  "$out/apple-touch-icon.png"
render icon-opaque.svg   192  192  "$out/android-chrome-192x192.png"
render icon-opaque.svg   512  512  "$out/android-chrome-512x512.png"
render og.svg            1200 630  "$out/images/og.png"

# ---------------------------------------------------------------------------
# favicon.ico, assembled from the two PNGs already rendered above.
#
# Hand-rolled rather than shelled out to a converter: the ICO container may
# embed PNG payloads verbatim (supported since Windows Vista), so this is a
# header plus the two files' bytes -- which keeps the dependency list to one
# rasteriser instead of also requiring ImageMagick or png-to-ico.
# ---------------------------------------------------------------------------
python3 - "$out" <<'PY'
import struct, sys, pathlib

out = pathlib.Path(sys.argv[1])
sizes = [16, 32]
images = [(s, (out / f"favicon-{s}x{s}.png").read_bytes()) for s in sizes]

# ICONDIR: reserved=0, type=1 (icon), count
header = struct.pack("<HHH", 0, 1, len(images))
offset = len(header) + 16 * len(images)

entries, payload = b"", b""
for size, data in images:
    # ICONDIRENTRY. A width/height byte of 0 would mean 256; 16 and 32 fit.
    entries += struct.pack(
        "<BBBBHHII",
        size, size,  # width, height
        0,           # palette colour count (0 = truecolour)
        0,           # reserved
        1,           # colour planes
        32,          # bits per pixel
        len(data),   # payload size
        offset,      # payload offset from start of file
    )
    payload += data
    offset += len(data)

(out / "favicon.ico").write_bytes(header + entries + payload)
print(f"    favicon.ico ({len(images)} sizes: {', '.join(str(s) for s in sizes)})")
PY

echo "==> wrote:"
cd "$out" && ls -1 favicon.svg favicon.ico favicon-*.png apple-touch-icon.png \
  android-chrome-*.png images/logo.svg images/logo-dark.svg images/og.png | sed 's/^/    site\/static\//'
