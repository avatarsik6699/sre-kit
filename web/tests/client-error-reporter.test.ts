import { describe, expect, it, vi } from "vitest";
import { createClientErrorReporter } from "~/shared/lib/client-errors/reporter";

describe("createClientErrorReporter", () => {
  it("forwards the first occurrence of an error to the sink", () => {
    const sink = vi.fn();
    const reporter = createClientErrorReporter(sink);

    reporter.report({ message: "boom" });

    expect(sink).toHaveBeenCalledTimes(1);
    expect(sink.mock.calls[0][0]).toMatchObject({ message: "boom", count: 1 });
  });

  it("dedupes repeated errors with the same fingerprint, incrementing count instead of re-sinking", () => {
    const sink = vi.fn();
    const reporter = createClientErrorReporter(sink);

    reporter.report({
      message: "boom",
      stack: "Error: boom\n  at foo (a.ts:1:1)",
    });
    reporter.report({
      message: "boom",
      stack: "Error: boom\n  at foo (a.ts:1:1)",
    });
    reporter.report({
      message: "boom",
      stack: "Error: boom\n  at foo (a.ts:1:1)",
    });

    expect(sink).toHaveBeenCalledTimes(1);
  });

  it("treats errors with different top stack frames as distinct fingerprints", () => {
    const sink = vi.fn();
    const reporter = createClientErrorReporter(sink);

    reporter.report({
      message: "boom",
      stack: "Error: boom\n  at foo (a.ts:1:1)",
    });
    reporter.report({
      message: "boom",
      stack: "Error: boom\n  at bar (b.ts:2:2)",
    });

    expect(sink).toHaveBeenCalledTimes(2);
  });
});
