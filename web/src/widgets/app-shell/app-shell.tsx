import { RailNav } from "~/widgets/rail-nav";
import type { AppShellTypes } from "./app-shell.types";
import styles from "./app-shell.module.css";

/** Rail nav + content area — the authenticated app's page chrome (docs/SPEC.md §5.3 layout). */
export const AppShell: React.FC<AppShellTypes.Props> = (props) => {
  return (
    <div className={styles.shell}>
      <RailNav />
      <div className={styles.content}>{props.children}</div>
    </div>
  );
};
