export namespace PageContainerTypes {
  export type Props = {
    children: React.ReactNode;
    /** Mantine Container size — defaults to "xl" for this dense, dashboard-style tool. */
    size?: string | number;
    className?: string;
  };
}
