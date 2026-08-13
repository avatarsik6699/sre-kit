import { Text, Title } from "@mantine/core";
import { mantineThemeConstants } from "~/shared/config/mantine-theme";
import type { TypographyTypes } from "./typography.types";

/**
 * The only place Mantine's Text/Title may be used directly (docs/FRONTEND_CONVENTIONS.md §8) —
 * ESLint forbids them everywhere else. Renders a Title when `variant="title"`, a Text otherwise;
 * `mono` switches to JetBrains Mono for literal data values per docs/SPEC.md §5.3.
 */
export const Typography: React.FC<TypographyTypes.Props> = (props) => {
  const fontFamily = props.mono
    ? mantineThemeConstants.fontFamilyMonospace
    : undefined;

  if (props.variant === "title") {
    return (
      <Title
        style={{ fontFamily }}
        order={props.order ?? 1}
        c={props.c}
        ta={props.ta}
        className={props.className}
      >
        {props.children}
      </Title>
    );
  }

  return (
    <Text
      style={{ fontFamily }}
      c={props.c}
      ta={props.ta}
      className={props.className}
    >
      {props.children}
    </Text>
  );
};
