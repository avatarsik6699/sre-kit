import { describe, expect, it } from "vitest";
import { Typography } from "~/shared/components/typography";
import { render, screen } from "./render";

describe("Typography", () => {
  it("renders a heading when variant is title", () => {
    render(
      <Typography variant="title" order={2}>
        Sources
      </Typography>,
    );
    expect(
      screen.getByRole("heading", { level: 2, name: "Sources" }),
    ).toBeInTheDocument();
  });

  it("renders body text by default", () => {
    render(<Typography>plain body copy</Typography>);
    expect(screen.getByText("plain body copy")).toBeInTheDocument();
  });

  it("applies the monospace font family for data values", () => {
    render(<Typography mono>vps-1.example.com</Typography>);
    const element = screen.getByText("vps-1.example.com");
    expect(element.style.fontFamily).toContain("JetBrains Mono");
  });
});
