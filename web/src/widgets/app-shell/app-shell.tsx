import { Box, Group } from "@mantine/core";
import { RailNav } from "~/widgets/rail-nav";
import type { AppShellTypes } from "./app-shell.types";

/** Rail nav + content area — the authenticated app's page chrome (docs/SPEC.md §5.3 layout). */
export const AppShell: React.FC<AppShellTypes.Props> = (props) => {
  return (
    <Group align="stretch" gap={0} wrap="nowrap" mih="100vh">
      <RailNav />
      <Box flex={1} p="lg" style={{ minWidth: 0 }}>
        {props.children}
      </Box>
    </Group>
  );
};
