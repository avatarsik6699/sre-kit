import { designTokens } from "~/shared/config/design-tokens";
import { Typography } from "~/shared/components/typography";
import classes from "./status-pulse.module.css";
import type { StatusPulseTypes } from "./status-pulse.types";

const statusColor: Record<StatusPulseTypes.Props["status"], string> = {
  ok: designTokens.statusOk,
  warn: designTokens.statusWarn,
  critical: designTokens.statusCritical,
  unreachable: designTokens.statusUnreachable,
};

/**
 * The status pulse (docs/SPEC.md §5.3) — identical dot+glow motif reused for source connectivity,
 * check status, alert severity, and adapter health. Status is never color-only: pair with a text
 * label (via `label`) or place next to one at the call site.
 */
export const StatusPulse: React.FC<StatusPulseTypes.Props> = (props) => {
  return (
    <span className={`${classes.wrapper} ${props.className ?? ""}`}>
      <span
        className={classes.dot}
        style={{
          ["--status-pulse-color" as string]: statusColor[props.status],
        }}
        role="img"
        aria-label={props.label ?? props.status}
      />
      {props.label ? <Typography mono>{props.label}</Typography> : null}
    </span>
  );
};
