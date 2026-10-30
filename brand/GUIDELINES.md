# Nullwell brand

Nullwell is a self-hosted RTBH controller: it collects blocklists and announces
blackhole routes to routers over BGP. Traffic falls in; nothing comes out.

## The mark

The **n-well**: the n of Nullwell drawn as a well in cross-section, with the
null point resting at the bottom. One idea, two stems, one arch, one dot.

| File | Use |
|---|---|
| `nullwell-horizontal.svg` | Default lockup on light backgrounds |
| `nullwell-horizontal-dark.svg` | Default lockup on dark backgrounds |
| `nullwell-stacked.svg` | Square-ish spaces (cards, splash) |
| `nullwell-symbol.svg` / `-dark.svg` | Symbol alone, 33 px and up |
| `nullwell-symbol-small.svg` | Symbol at 16–32 px (heavier stems, larger dot) |
| `nullwell-symbol-app-icon.svg` | App icon / avatar tile |
| `*-black.svg`, `*-white.svg`, `*-mono-1a2028.svg` | One-colour printing, embroidery, etching |
| `nullwell-wordmark.svg` | Wordmark alone, when the symbol is already nearby |
| `icons/` | favicon.ico, favicon.svg, PNG icons, maskable icon |

## Colour

| Name | Hex | RGB | Role |
|---|---|---|---|
| Graphite | `#1A2028` | 26 32 40 | Mark and wordmark on light |
| Signal | `#00736F` | 0 115 111 | Null point on light |
| Signal bright | `#52CEC4` | 82 206 196 | Null point on dark |
| Night | `#0D1116` | 13 17 22 | Dark ground, icon tile |
| Mist | `#E4E8ED` | 228 232 237 | Mark and wordmark on dark |

The teal is the only accent and belongs to the null point. Never colour the n
teal, and never add a second accent. In one-colour use the dot takes the same
colour as the n.

These match the panel tokens in `web/src/index.css` (`--foreground`,
`--signal`, dark `--background`).

## Typography

Wordmark: IBM Plex Sans SemiBold, lowercase, tracking -12/1000, converted to
outlines. Interface text: IBM Plex Sans; IPs, CIDRs and ASNs: IBM Plex Mono.
Plex is licensed under the SIL Open Font License, which permits logo use.

## Clear space and size

- Clear space on every side: the diameter of the null point (48 of 256 units).
- Minimum size: symbol 16 px (use the small cut up to 32 px); horizontal
  lockup 96 px wide; stacked lockup 48 px wide.
- In the horizontal lockup the symbol height equals the wordmark's ascender
  height; keep the gap as drawn.

## Don't

- Rotate, flip, outline, or add shadows, glows or gradients.
- Move, resize or recolour the dot independently of the n.
- Set "nullwell" in another font, in capitals, or as "NullWell".
- Place the light version on dark grounds (use `-dark` or `-white`).
- Squeeze the lockup or put it inside a shield, globe or other container.

## Status

Name and mark are not trademark-cleared. Before public launch, search PDKI
(Indonesia) and the WIPO Global Brand Database for classes 9 and 42.
