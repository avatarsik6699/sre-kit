import { describe, expect, it, vi } from "vitest";
import { render, screen } from "./render";
import { TelemetrySummary } from "~/widgets/telemetry-summary";

vi.mock("~/entities/telemetry", () => ({
  useMetricsQuery: () => ({
    data: [
      {
        name: "analytics.pageviews",
        ts: "2026-08-21T12:00:00Z",
        value: 1200,
        labels: { traffic_class: "browser_analytics" },
      },
      {
        name: "analytics.country_count",
        ts: "2026-08-21T12:00:00Z",
        value: 12,
        labels: { dimension: "country", value: "DE" },
      },
    ],
    isPending: false,
    isError: false,
  }),
  useChecksQuery: () => ({ data: [], isPending: false, isError: false }),
}));

describe("TelemetrySummary", () => {
  it("groups and labels metrics from the adapter presentation schema", () => {
    render(
      <TelemetrySummary
        sourceId="source-1"
        presentationSchema={{
          groups: [
            { id: "overview", title: "Overview", order: 10 },
            { id: "audience", title: "Audience", order: 20 },
          ],
          measurements: [
            {
              name: "analytics.pageviews",
              title: "Pageviews",
              group: "overview",
              unit: "count",
              visualization: "stat",
            },
            {
              name: "analytics.country_count",
              title: "Countries",
              group: "audience",
              unit: "count",
              visualization: "table",
            },
          ],
        }}
      />,
    );

    expect(screen.getByRole("heading", { name: "Overview" })).toBeVisible();
    expect(screen.getByText("Pageviews")).toBeVisible();
    expect(screen.getByText("1,200")).toBeVisible();
    expect(screen.getByRole("heading", { name: "Audience" })).toBeVisible();
    expect(screen.getByText("dimension: country · value: DE")).toBeVisible();
  });
});
