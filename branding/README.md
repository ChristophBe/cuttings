# Branding

The authored sources for the cuttings mark, plus the script that renders them into the web
assets the site serves. **Everything in `site/static/` listed below is generated — edit the
sources here, never the output.**

## The mark

A single leaf, drawn with three strokes on a 32×32 grid: one straight line that is both the
stem and the leaf's midrib, and two mirrored quadratic curves forming the blade. The curves
are quadratic rather than cubic on purpose — a quadratic's tangent at each end points straight
at its single control point, so the blade meets the midrib in a true cusp at both the tip and
the base. An earlier upright, cubic version domed over at both ends and read as a lollipop.

Colours are the site's existing accent, defined at the top of `site/assets/css/custom.css`:

| | Colour | CSS |
|---|---|---|
| Light | `#0AB895` | `hsl(168 90% 38%)` |
| Dark | `#2BEEC7` | `hsl(168 85% 55%)` |

The same leaf appears as ASCII art in the CLI's `cuttings --help` output — see the `banner`
constant in [`cmd/banner.go`](../cmd/banner.go).

## Sources

| File | Purpose |
|---|---|
| `mark.svg` | The mark in the light-mode accent. Source for the navbar logo and every transparent raster. |
| `mark-dark.svg` | Same geometry in the dark-mode accent. Hextra swaps the navbar logo as a light/dark `<img>` pair, so the dark variant has to be its own file. |
| `favicon.svg` | Same geometry again, but carrying an embedded `prefers-color-scheme` media query. A favicon renders outside the page's stylesheet, so the site's class-based dark toggle cannot reach it — only an embedded query can. |
| `icon-opaque.svg` | The mark on a solid plate, scaled to 78%. iOS and Android composite home-screen icons onto their own background and apply their own corner mask, so these must not be transparent and must not run to the edge. |
| `og.svg` | The 1200×630 social share card (`og:image` / `twitter:image`). |

## Regenerating

```bash
make generate-branding      # from the repository root
```

Requires `python3` (used to assemble the multi-size `favicon.ico`) and **one** SVG rasteriser.
The script picks the first it finds:

```bash
brew install librsvg        # rsvg-convert — recommended, works offline
brew install imagemagick    # magick
# otherwise falls back to `npx @resvg/resvg-js-cli`, which needs network on first run
```

Outputs are committed, so CI never runs this script and no CI job needs a rasteriser
installed.

## Generated output

| Output | Source |
|---|---|
| `site/static/favicon.svg` | `favicon.svg` (copied) |
| `site/static/images/logo.svg` | `mark.svg` (copied) |
| `site/static/images/logo-dark.svg` | `mark-dark.svg` (copied) |
| `site/static/favicon-16x16.png`, `favicon-32x32.png` | `mark.svg` |
| `site/static/favicon.ico` | assembled from the two PNGs above |
| `site/static/apple-touch-icon.png` (180) | `icon-opaque.svg` |
| `site/static/android-chrome-192x192.png`, `-512x512.png` | `icon-opaque.svg` |
| `site/static/images/og.png` | `og.svg` |

Those filenames are not arbitrary: they are exactly the paths the Hextra theme's
`_partials/favicons.html` already requests. Hugo resolves project files under `site/static/`
ahead of the theme's own, so dropping these in is what displaces Hextra's default branding —
no template override is needed. `site/static/site.webmanifest` is hand-written rather than
generated, but exists for the same reason: the theme ships one whose `name` is `"Hextra"`.
