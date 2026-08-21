import type { PresentationSchema } from "~/entities/adapter";

export namespace TelemetrySummaryTypes {
  export type Props = {
    sourceId: string;
    title?: string;
    presentationSchema?: PresentationSchema;
  };
}
