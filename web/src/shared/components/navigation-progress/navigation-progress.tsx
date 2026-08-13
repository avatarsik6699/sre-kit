import { NavigationProgress as MantineNavigationProgress } from "@mantine/nprogress";

/**
 * Top-of-page navigation progress bar. Mounted once in the root document (I10); driven by
 * `nprogress.start()`/`nprogress.complete()` (re-exported below) from the router's navigation
 * lifecycle, not from this component itself.
 */
export const NavigationProgress: React.FC = () => {
  return <MantineNavigationProgress />;
};

export { nprogress } from "@mantine/nprogress";
