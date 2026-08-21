import { useState } from "react";
import { ActionIcon, Stack } from "~/shared/ui";
import { Link, useRouterState } from "@tanstack/react-router";
import { designTokens } from "~/shared/config/design-tokens";
import styles from "./rail-nav.module.css";

const NAV_ITEMS = [
  { to: "/", label: "Dashboard", icon: "◧" },
  { to: "/sources", label: "Sources", icon: "▤" },
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
      p="xs"
      gap={4}
      className={styles.nav}
      style={{
        "--rail-width": collapsed ? "56px" : "200px",
        "--rail-border": designTokens.bgSurfaceRaised,
      } as React.CSSProperties}
    >
      <Stack gap={4} className={styles.items}>
        {NAV_ITEMS.map((item) => (
          <Link
            key={item.to}
            to={item.to}
            className={`${styles.link} ${routerState.location.pathname === item.to ? styles.active : ""}`}
            aria-label={collapsed ? item.label : undefined}
          >
            <span aria-hidden="true">{item.icon}</span>
            {collapsed ? null : <span>{item.label}</span>}
          </Link>
        ))}
      </Stack>
      <ActionIcon
        className={styles.toggle}
        variant="subtle"
        aria-label={collapsed ? "Expand navigation" : "Collapse navigation"}
        onClick={() => setCollapsed((prev) => !prev)}
      >
        {collapsed ? "»" : "«"}
      </ActionIcon>
    </Stack>
  );
};
