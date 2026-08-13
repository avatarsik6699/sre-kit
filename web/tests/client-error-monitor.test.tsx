import { describe, expect, it, vi } from "vitest";
import { ClientErrorMonitor } from "~/shared/components/client-error-monitor";
import { render } from "./render";

describe("ClientErrorMonitor", () => {
  it("renders nothing and captures a window error without crashing", () => {
    const consoleError = vi
      .spyOn(console, "error")
      .mockImplementation(() => {});
    const { container } = render(<ClientErrorMonitor />);

    // MantineProvider injects its own <style> tags into the tree, so the container isn't
    // literally empty — assert the monitor itself renders no visible (non-style) content instead.
    const visibleChildren = Array.from(container.children).filter(
      (el) => el.tagName !== "STYLE",
    );
    expect(visibleChildren).toHaveLength(0);

    window.dispatchEvent(
      new ErrorEvent("error", { error: new Error("boom"), message: "boom" }),
    );
    expect(consoleError).toHaveBeenCalledWith(
      "[client-error]",
      expect.objectContaining({ message: "boom" }),
    );

    consoleError.mockRestore();
  });
});
