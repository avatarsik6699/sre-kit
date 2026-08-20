import { useEffect } from "react";
import {
  attachBrowserErrorListeners,
  consoleSink,
  createClientErrorReporter,
} from "~/shared/lib/client-errors";

const reporter = createClientErrorReporter(consoleSink);

/**
 * Mounted once in the root document (docs/changes/archive/01-core-skeleton.md I10). Renders nothing —
 * pure side-effect component that wires up global client-error capture for the app's lifetime.
 */
export const ClientErrorMonitor: React.FC = () => {
  useEffect(function attachClientErrorListenersFx() {
    return attachBrowserErrorListeners(reporter);
  }, []);

  return null;
};
