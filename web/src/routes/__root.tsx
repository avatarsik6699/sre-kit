// Root route (docs/changes/01-core-skeleton.md I10): document shell, dark-default Mantine theme
// (docs/SPEC.md §5.3), and the two app-wide side-effect mounts — ClientErrorMonitor (I9) and the
// navigation progress bar (I8). No page/feature content lives here — real pages start at M4.
import {
  ColorSchemeScript,
  MantineProvider,
  mantineHtmlProps,
} from "@mantine/core";
import "@mantine/core/styles.css";
import "@mantine/nprogress/styles.css";
import {
  HeadContent,
  Outlet,
  Scripts,
  createRootRouteWithContext,
} from "@tanstack/react-router";
import type { QueryClient } from "@tanstack/react-query";
import type React from "react";
import { ClientErrorMonitor } from "~/shared/components/client-error-monitor";
import { NavigationProgress } from "~/shared/components/navigation-progress";
import { mantineTheme } from "~/shared/config/mantine-theme";

// Typed router context (queryClient, wired in router.tsx) — routes/_authenticated.tsx's
// beforeLoad reads context.queryClient to warm/guard the sources query before rendering.
export type RouterContext = {
  queryClient: QueryClient;
};

export const Route = createRootRouteWithContext<RouterContext>()({
  head: () => ({
    meta: [
      { charSet: "utf-8" },
      { name: "viewport", content: "width=device-width, initial-scale=1" },
      { title: "sre-kit" },
    ],
  }),
  component: RootComponent,
  shellComponent: RootDocument,
});

function RootComponent() {
  return <Outlet />;
}

type RootDocumentProps = { children: React.ReactNode };

function RootDocument(props: RootDocumentProps) {
  return (
    <html lang="en" {...mantineHtmlProps}>
      <head>
        <ColorSchemeScript defaultColorScheme="dark" />
        <HeadContent />
      </head>
      <body>
        <MantineProvider theme={mantineTheme} defaultColorScheme="dark">
          <NavigationProgress />
          <ClientErrorMonitor />
          {props.children}
        </MantineProvider>
        <Scripts />
      </body>
    </html>
  );
}
