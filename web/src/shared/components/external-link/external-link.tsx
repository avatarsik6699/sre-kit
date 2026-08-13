import { Anchor } from "@mantine/core";
import type { ExternalLinkTypes } from "./external-link.types";

/**
 * The only place a raw `<a>` (or Mantine's Anchor) may render — ESLint forbids both elsewhere.
 * TanStack Router's `Link` remains the primitive for internal navigation, unwrapped
 * (docs/FRONTEND_CONVENTIONS.md §8) — this component is for links leaving the app only, so it
 * always opens in a new tab with `rel="noopener noreferrer"`.
 */
export const ExternalLink: React.FC<ExternalLinkTypes.Props> = (props) => {
  return (
    <Anchor
      href={props.href}
      target="_blank"
      rel="noopener noreferrer"
      className={props.className}
    >
      {props.children}
    </Anchor>
  );
};
