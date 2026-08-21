// Legacy import path retained while consumers migrate; owns framework-independent design tokens.
// Palette, typography, and spacing tokens are the design system fixed in docs/SPEC.md §5.3 —
// dark-default, status colors semantic and never repurposed for chrome, the "status pulse" motif
// reused across source connectivity / check status / alert severity / adapter health.
export const designTokens = {
  bgCanvas: "#0B0E14",
  bgSurface: "#12161F",
  bgSurfaceRaised: "#1A1F2B",
  textPrimary: "#E6E9F0",
  textMuted: "#8992A6",
  accentPrimary: "#6C8EFF",
  statusOk: "#3DD68C",
  statusWarn: "#F5B94A",
  statusCritical: "#F2545B",
  statusUnreachable: "#5B6478",
  fontFamily: "'Instrument Sans', sans-serif",
  fontFamilyMonospace: "'JetBrains Mono', monospace",
  spacingUnitPx: 4,
} as const;
