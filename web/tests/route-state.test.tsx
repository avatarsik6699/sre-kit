import { userEvent } from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import {
  RouteError,
  RouteNotFound,
  RoutePending,
} from "~/shared/components/route-state";
import { render, screen } from "./render";

describe("RoutePending", () => {
  it("renders a loading indicator", () => {
    render(<RoutePending />);
    expect(screen.getByText("Loading…")).toBeInTheDocument();
  });
});

describe("RouteError", () => {
  it("shows the error message", () => {
    render(<RouteError error={new Error("boom")} />);
    expect(screen.getByText("boom")).toBeInTheDocument();
  });

  it("calls reset when the retry button is clicked", async () => {
    const reset = vi.fn();
    render(<RouteError error={new Error("boom")} reset={reset} />);
    await userEvent
      .setup()
      .click(screen.getByRole("button", { name: "Retry" }));
    expect(reset).toHaveBeenCalledTimes(1);
  });
});

describe("RouteNotFound", () => {
  it("renders a not-found message", () => {
    render(<RouteNotFound />);
    expect(screen.getByText("Page not found")).toBeInTheDocument();
  });
});
