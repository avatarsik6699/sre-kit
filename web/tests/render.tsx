// Vitest render helper for repository-owned semantic components.
import { render as testingLibraryRender } from "@testing-library/react";
import type React from "react";

export function render(ui: React.ReactNode) {
  return testingLibraryRender(<>{ui}</>, {
    wrapper: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  });
}

export { screen } from "@testing-library/react";
