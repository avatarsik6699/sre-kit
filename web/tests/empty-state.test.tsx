import { describe, expect, it } from "vitest";
import { EmptyState } from "~/shared/components/empty-state";
import { render, screen } from "./render";

describe("EmptyState", () => {
  it("renders the title and optional description", () => {
    render(
      <EmptyState
        title="No sources yet"
        description="Add one to get started."
      />,
    );
    expect(screen.getByText("No sources yet")).toBeInTheDocument();
    expect(screen.getByText("Add one to get started.")).toBeInTheDocument();
  });

  it("renders without a description", () => {
    render(<EmptyState title="No sources yet" />);
    expect(screen.getByText("No sources yet")).toBeInTheDocument();
  });

  it("renders the action node", () => {
    render(
      <EmptyState
        title="No sources yet"
        action={<button type="button">Add source</button>}
      />,
    );
    expect(
      screen.getByRole("button", { name: "Add source" }),
    ).toBeInTheDocument();
  });
});
