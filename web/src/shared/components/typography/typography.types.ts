import type React from "react";

export namespace TypographyTypes {
  export type Props = {
    children: React.ReactNode;
    /** "title" renders a heading (requires `order`); default "text" renders body copy. */
    variant?: "title" | "text";
    /** Heading level — required when variant is "title", ignored otherwise. */
    order?: 1 | 2 | 3 | 4 | 5 | 6;
    /**
     * Renders in JetBrains Mono instead of Instrument Sans — reserved for literal data values
     * (hostnames, source IDs, metric numbers, timestamps, log/event lines) per docs/SPEC.md §5.3.
     */
    mono?: boolean;
    c?: string;
    ta?: "left" | "center" | "right";
    className?: string;
  };
}
