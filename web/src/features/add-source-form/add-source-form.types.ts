import type { AdapterManifest } from "~/entities/adapter";

export namespace AddSourceFormTypes {
  export type Props = {
    adapter: AdapterManifest;
    isSubmitting: boolean;
    onSubmit: (config: Record<string, unknown>) => void;
  };
}

/** One property of the adapter's config_schema (JSON Schema subset the manifest declares). */
export type ConfigSchemaProperty = {
  type?: string;
  format?: string;
  enum?: string[];
  default?: unknown;
  minimum?: number;
  maximum?: number;
  description?: string;
};

export type ConfigSchema = {
  properties?: Record<string, ConfigSchemaProperty>;
  required?: string[];
};
