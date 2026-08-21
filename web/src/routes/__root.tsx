// Root route: document shell and the app-wide client-error boundary.
// (docs/SPEC.md §5.3), and the two app-wide side-effect mounts — ClientErrorMonitor (I9) and the
// navigation progress bar (I8). No page/feature content lives here — real pages start at M4.
import "~/shared/styles/global.css";
import {
  HeadContent,
  Outlet,
  Scripts,
  createRootRouteWithContext,
} from "@tanstack/react-router";
import type { QueryClient } from "@tanstack/react-query";
import type React from "react";
import { ClientErrorMonitor } from "~/shared/components/client-error-monitor";

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
    <html lang="en">
      <head>
        <HeadContent />
        <link
          rel="icon"
          href="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 64 64'%3E%3Crect width='64' height='64' rx='12' fill='%236387f5'/%3E%3Cpath d='M18 18h8v28h-8zm20 0h8v28h-8z' fill='%230b0e14'/%3E%3C/svg%3E"
        />
      </head>
      <body>
        <ClientErrorMonitor />
        {props.children}
        <Scripts />
      </body>
    </html>
  );
}
