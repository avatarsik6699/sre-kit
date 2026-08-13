// Owns all global Mantine theme values and component defaults (docs/FRONTEND_CONVENTIONS.md §8).
// Palette, typography, and spacing tokens are the design system fixed in docs/SPEC.md §5.3 —
// dark-default, status colors semantic and never repurposed for chrome, the "status pulse" motif
// reused across source connectivity / check status / alert severity / adapter health.
import { colorsTuple, createTheme } from "@mantine/core";

export const mantineThemeConstants = {
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

export const mantineTheme = createTheme({
  primaryColor: "accentPrimary",
  primaryShade: { light: 6, dark: 8 },
  fontFamily: mantineThemeConstants.fontFamily,
  fontFamilyMonospace: mantineThemeConstants.fontFamilyMonospace,
  headings: {
    fontFamily: mantineThemeConstants.fontFamily,
    fontWeight: "600",
  },
  defaultRadius: "md",
  colors: {
    accentPrimary: colorsTuple(mantineThemeConstants.accentPrimary),
    statusOk: colorsTuple(mantineThemeConstants.statusOk),
    statusWarn: colorsTuple(mantineThemeConstants.statusWarn),
    statusCritical: colorsTuple(mantineThemeConstants.statusCritical),
    statusUnreachable: colorsTuple(mantineThemeConstants.statusUnreachable),
  },
  other: {
    bgCanvas: mantineThemeConstants.bgCanvas,
    bgSurface: mantineThemeConstants.bgSurface,
    bgSurfaceRaised: mantineThemeConstants.bgSurfaceRaised,
    textPrimary: mantineThemeConstants.textPrimary,
    textMuted: mantineThemeConstants.textMuted,
  },
});
