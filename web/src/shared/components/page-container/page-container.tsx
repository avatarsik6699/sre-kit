import type { PageContainerTypes } from "./page-container.types";

/** The only place repository-owned's Container may be used directly — ESLint forbids it elsewhere. */
export const PageContainer: React.FC<PageContainerTypes.Props> = (props) => {
  return (
    <main
      className={props.className}
      style={{ width: "min(1440px, 100%)", margin: "0 auto" }}
    >
      {props.children}
    </main>
  );
};
