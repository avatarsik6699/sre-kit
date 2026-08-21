export namespace PageContainerTypes {
  export type Props = {
    children: React.ReactNode;
    /** repository-owned Container size — defaults to "xl" for this dense, dashboard-style tool. */
    size?: string | number;
    className?: string;
  };
}
