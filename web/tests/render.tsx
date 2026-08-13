// Themed Vitest render helper (docs/FRONTEND_CONVENTIONS.md §9): wraps units under test in the
// production Mantine theme so policy components (Typography, etc.) render as they do in the app.
import { MantineProvider } from "@mantine/core";
import { render as testingLibraryRender } from "@testing-library/react";
import type React from "react";
import { mantineTheme } from "~/shared/config/mantine-theme";

export function render(ui: React.ReactNode) {
  return testingLibraryRender(<>{ui}</>, {
    wrapper: ({ children }: { children: React.ReactNode }) => (
      <MantineProvider theme={mantineTheme} env="test">
        {children}
      </MantineProvider>
    ),
  });
}

export { screen } from "@testing-library/react";
