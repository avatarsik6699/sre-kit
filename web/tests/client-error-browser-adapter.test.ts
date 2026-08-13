import { afterEach, describe, expect, it, vi } from "vitest";
import { attachBrowserErrorListeners } from "~/shared/lib/client-errors/browser-adapter";
import type { ClientErrorReporter } from "~/shared/lib/client-errors/reporter";

describe("attachBrowserErrorListeners", () => {
  let detach: (() => void) | null = null;

  afterEach(() => {
    detach?.();
    detach = null;
  });

  it("reports a window error event", () => {
    const report = vi.fn();
    const reporter: ClientErrorReporter = { report };
    detach = attachBrowserErrorListeners(reporter);

    const error = new Error("boom");
    window.dispatchEvent(
      new ErrorEvent("error", { error, message: error.message }),
    );

    expect(report).toHaveBeenCalledWith(
      expect.objectContaining({ message: "boom" }),
    );
  });

  it("reports an unhandled promise rejection", () => {
    const report = vi.fn();
    const reporter: ClientErrorReporter = { report };
    detach = attachBrowserErrorListeners(reporter);

    // jsdom's PromiseRejectionEvent requires a genuinely rejected (and handled-by-us) promise.
    const rejected = Promise.reject(new Error("rejected"));
    rejected.catch(() => {});
    window.dispatchEvent(
      new PromiseRejectionEvent("unhandledrejection", {
        promise: rejected,
        reason: new Error("rejected"),
      }),
    );

    expect(report).toHaveBeenCalledWith(
      expect.objectContaining({ message: "rejected" }),
    );
  });

  it("stops reporting after being detached", () => {
    const report = vi.fn();
    const reporter: ClientErrorReporter = { report };
    const detachFn = attachBrowserErrorListeners(reporter);
    detachFn();

    // With no listener left to preventDefault() it, jsdom's own default action for a dispatched
    // `error` event on window is to report it as an uncaught exception — suppress that default
    // action here (a real, unhandled runtime error would do the same in a browser) so the test
    // only asserts what it means to: our reporter isn't called anymore.
    const suppressDefaultAction = (event: Event) => event.preventDefault();
    window.addEventListener("error", suppressDefaultAction, { once: true });
    window.dispatchEvent(
      new ErrorEvent("error", { error: new Error("boom"), message: "boom" }),
    );

    expect(report).not.toHaveBeenCalled();
  });
});
