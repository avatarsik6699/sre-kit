import { Button, Loader, Stack } from "@mantine/core";
import { Typography } from "~/shared/components/typography";

// Three tiny, tightly-coupled route-state indicators wired as one trio onto TanStack Router's
// defaultPendingComponent/defaultErrorComponent/defaultNotFoundComponent (docs/changes/01-core-skeleton.md
// I10) — kept together in one file per this change's Files list rather than split one-per-file,
// since neither is meaningful without the others as a set.

/** Router-wide defaultPendingComponent: shown while a route's loader is in flight. */
export const RoutePending: React.FC = () => {
  return (
    <Stack align="center" justify="center" mih={200} gap="sm">
      <Loader />
      <Typography c="dimmed">Loading…</Typography>
    </Stack>
  );
};

export type RouteErrorProps = {
  error: Error;
  reset?: () => void;
};

/** Router-wide defaultErrorComponent: shown when a route's loader/component throws. */
export const RouteError: React.FC<RouteErrorProps> = (props) => {
  return (
    <Stack align="center" justify="center" mih={200} gap="sm">
      <Typography variant="title" order={3}>
        Something went wrong
      </Typography>
      <Typography c="dimmed" mono>
        {props.error.message}
      </Typography>
      {props.reset ? (
        <Button onClick={props.reset} variant="light">
          Retry
        </Button>
      ) : null}
    </Stack>
  );
};

/**
 * Router-wide defaultNotFoundComponent: shown for an unmatched route. No "go home" link yet —
 * this change scaffolds no page routes at all (docs/changes/01-core-skeleton.md § Do NOT touch);
 * add one once a real index route exists at M4.
 */
export const RouteNotFound: React.FC = () => {
  return (
    <Stack align="center" justify="center" mih={200} gap="sm">
      <Typography variant="title" order={3}>
        Page not found
      </Typography>
    </Stack>
  );
};
