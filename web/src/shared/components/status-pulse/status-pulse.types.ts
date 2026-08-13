export type StatusPulseStatus = "ok" | "warn" | "critical" | "unreachable";

export namespace StatusPulseTypes {
  export type Props = {
    status: StatusPulseStatus;
    /** Visible text label rendered next to the dot — status must never be color-only (SPEC §5.3). */
    label?: string;
    className?: string;
  };
}
