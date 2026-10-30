# RTBH Portal design direction

Owner brief (2026-10-30): calm NOC tool, dense, dark-friendly but light-friendly too.
Update: owner (wears -5 glasses) found 13px too small; base raised to 15px.

## Identity

- Audience: network operators watching a blackhole feed, often on a NOC wall or a
  second monitor, sometimes from a phone during an incident.
- The one question every screen answers first: is the blocklist live, and are
  routers receiving it?
- Calm: nothing moves or glows unless state changed. Colour means state, not decoration.
- Readable first, then dense: 15px body text, 13px minimum, tight table rows.
  A Large text switch (sidebar) scales the whole UI 112.5% for low-vision users.

## Palette

- Neutrals: cool graphite (slight blue tint) for text, surfaces and borders.
- Accent (one): signal teal = live / announcing / healthy. Used for the feed
  status, the active nav marker and "on" states. Never for decoration.
- State only: amber = stale or needs attention, red = error or destructive.
- Light and dark themes are equal citizens; follow the OS by default with a
  manual System / Light / Dark switch.

## Typography

- IBM Plex Sans for UI: built for technical interfaces, clear at small sizes.
- IBM Plex Mono for IPs, CIDRs, ASNs: unambiguous 0/O and 1/l.
- Tabular numerals everywhere counts appear.

## Dials

ENERGY 1 / RHYTHM 1 / MOTION 1. Uniform, predictable layout on purpose; hover
and focus states only, no entrance animations.
