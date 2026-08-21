import { designTokens } from "~/shared/config/design-tokens";
import type { TypographyTypes } from "./typography.types";

/**
 * The only place repository-owned's Text/Title may be used directly (docs/FRONTEND_CONVENTIONS.md §8) —
 * ESLint forbids them everywhere else. Renders a Title when `variant="title"`, a Text otherwise;
 * `mono` switches to JetBrains Mono for literal data values per docs/SPEC.md §5.3.
 */
export const Typography: React.FC<TypographyTypes.Props> = (props) => {
  const fontFamily = props.mono ? designTokens.fontFamilyMonospace : undefined;

  if (props.variant === "title") {
    const Heading = `h${props.order ?? 1}` as
      "h1" | "h2" | "h3" | "h4" | "h5" | "h6";
    return (
      <Heading
        style={{
          fontFamily,
          color: props.c === "dimmed" ? "var(--muted)" : props.c,
          textAlign: props.ta,
        }}
        className={props.className}
      >
        {props.children}
      </Heading>
    );
  }

  return (
    <p
      style={{
        fontFamily,
        color: props.c === "dimmed" ? "var(--muted)" : props.c,
        textAlign: props.ta,
      }}
      className={props.className}
    >
      {props.children}
    </p>
  );
};
