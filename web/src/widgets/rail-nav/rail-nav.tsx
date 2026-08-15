import { useState } from "react";
import { ActionIcon, NavLink, Stack } from "@mantine/core";
import { Link, useRouterState } from "@tanstack/react-router";
import { mantineThemeConstants } from "~/shared/config/mantine-theme";

const NAV_ITEMS = [
  { to: "/", label: "Dashboard", icon: "◧" },
  { to: "/sources", label: "Sources", icon: "▤" },
  { to: "/hosts", label: "Hosts", icon: "▣" },
  { to: "/notifications", label: "Notifications", icon: "◔" },
] as const;

/** Left icon+label collapsible rail nav (docs/SPEC.md §5.3) — persistent side nav for a tool used
 * all day, over a marketing-style top nav. */
export const RailNav: React.FC = () => {
  const [collapsed, setCollapsed] = useState(false);
  const routerState = useRouterState();

  return (
    <Stack
      component="nav"
      justify="space-between"
      w={collapsed ? 56 : 200}
      p="xs"
      gap={4}
      style={{
        borderRight: `1px solid ${mantineThemeConstants.bgSurfaceRaised}`,
        transition: "width 150ms ease",
        flexShrink: 0,
      }}
    >
      <Stack gap={4}>
        {NAV_ITEMS.map((item) => (
          <NavLink
            key={item.to}
            component={Link}
            to={item.to}
            label={collapsed ? undefined : item.label}
            leftSection={item.icon}
            active={routerState.location.pathname === item.to}
          />
        ))}
      </Stack>
      <ActionIcon
        variant="subtle"
        aria-label={collapsed ? "Expand navigation" : "Collapse navigation"}
        onClick={() => setCollapsed((prev) => !prev)}
      >
        {collapsed ? "»" : "«"}
      </ActionIcon>
    </Stack>
  );
};
