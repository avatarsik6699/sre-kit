import { describe, expect, it } from "vitest";
import { addSourceFormUtils } from "~/features/add-source-form/add-source-form";
import type { ConfigSchema } from "~/features/add-source-form/add-source-form.types";

const schema: ConfigSchema = {
  properties: {
    host: { type: "string" },
    port: { type: "integer", default: 22, minimum: 1, maximum: 65535 },
  },
  required: ["host"],
};

describe("addSourceFormUtils.buildInitialValues", () => {
  it("prefills only fields with a schema default", () => {
    expect(addSourceFormUtils.buildInitialValues(schema)).toEqual({ port: 22 });
  });
});

describe("addSourceFormUtils.validate", () => {
  it("reports a missing required field", () => {
    expect(addSourceFormUtils.validate(schema, { port: 22 })).toEqual([
      "host is required",
    ]);
  });

  it("reports an out-of-range numeric field", () => {
    expect(
      addSourceFormUtils.validate(schema, { host: "x", port: 99999 }),
    ).toEqual(["port must be <= 65535"]);
  });

  it("passes with valid values", () => {
    expect(
      addSourceFormUtils.validate(schema, { host: "x", port: 22 }),
    ).toEqual([]);
  });
});
