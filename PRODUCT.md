# sre-kit product contract

## Product

sre-kit is a self-hosted observability and analytics workspace for one operator managing several
small applications and hosts. It answers two questions quickly: what needs attention now, and what
changed in the traffic or system signals behind it. It observes targets but never deploys,
configures or repairs them.

## Users and jobs

The primary user is a solo developer/SRE. They need to scan multiple projects, distinguish missing
data from healthy data, inspect every signal an integration exposes, and move from an overview to
the underlying Source without learning adapter-specific screens.

## Experience principles

- Dark, compact and calm. Decoration yields to readable values, labels and state.
- Overview first, evidence one click away. Dashboard groups signals by Project and meaning;
  Source detail exposes the complete declared measurement set.
- Unknown stays unknown. Traffic classifiers never relabel unclassified sessions as humans.
- Schema over special cases. Integrations describe presentation; UI owns reusable rendering.
- Charts support comparison, not decoration. Every canvas plot has a textual/table alternative.
- Status is never color-only. Loading, empty, stale, unreachable and failed states are distinct.

## Visual direction

Ink-black canvas, low-contrast graphite panels, off-white text and restrained blue focus/accent.
Green, amber and red are reserved for health. Typography is system sans for navigation and labels,
with a system monospace stack for numbers, identifiers and timestamps. Borders and spacing create
hierarchy; gradients, glass effects, oversized headings and ornamental animation are excluded.

## Success

An operator can identify unhealthy Projects/Sources and the dominant traffic/acquisition/content
changes from the first viewport, then inspect complete labeled telemetry without opening Umami,
Beszel or raw logs. Keyboard navigation and narrow-width use remain first-class.
