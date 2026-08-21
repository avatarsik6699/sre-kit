import { useState } from "react";
import {
  Alert,
  Button,
  Checkbox,
  Group,
  NumberInput,
  PasswordInput,
  Select,
  Stack,
  TextInput,
} from "~/shared/ui";
import { Typography } from "~/shared/components/typography";
import type {
  AddSourceFormTypes,
  ConfigSchema,
  ConfigSchemaProperty,
} from "./add-source-form.types";

export const addSourceFormUtils = {
  buildInitialValues(schema: ConfigSchema): Record<string, unknown> {
    const values: Record<string, unknown> = {};
    for (const [key, property] of Object.entries(schema.properties ?? {})) {
      if (property.default !== undefined) {
        values[key] = property.default;
      }
    }
    return values;
  },

  /**
   * Client-side "test connection": validates the entered config against the manifest's
   * config_schema (required fields present, numeric bounds respected). No backend endpoint
   * exists yet to actually probe the target (SPEC §4's API table has no test-connection route) —
   * see docs/changes/archive/04-minimal-ui.md § Implementation Notes for the follow-up.
   */
  validate(schema: ConfigSchema, values: Record<string, unknown>): string[] {
    const errors: string[] = [];
    for (const key of schema.required ?? []) {
      const value = values[key];
      if (value === undefined || value === "") {
        errors.push(`${key} is required`);
      }
    }
    for (const [key, property] of Object.entries(schema.properties ?? {})) {
      const value = values[key];
      if (typeof value !== "number") {
        continue;
      }
      if (property.minimum !== undefined && value < property.minimum) {
        errors.push(`${key} must be >= ${property.minimum}`);
      }
      if (property.maximum !== undefined && value > property.maximum) {
        errors.push(`${key} must be <= ${property.maximum}`);
      }
    }
    return errors;
  },
};

type FieldProps = {
  name: string;
  property: ConfigSchemaProperty;
  required: boolean;
  value: unknown;
  onChange: (value: unknown) => void;
};

const ConfigField: React.FC<FieldProps> = (props) => {
  if (props.property.enum) {
    return (
      <Select
        label={props.name}
        description={props.property.description}
        required={props.required}
        data={props.property.enum}
        value={typeof props.value === "string" ? props.value : null}
        onChange={(value) => props.onChange(value)}
      />
    );
  }
  if (props.property.format === "secret") {
    return (
      <PasswordInput
        label={props.name}
        description={props.property.description}
        required={props.required}
        value={typeof props.value === "string" ? props.value : ""}
        onChange={(event) => props.onChange(event.currentTarget.value)}
      />
    );
  }
  if (props.property.type === "integer" || props.property.type === "number") {
    return (
      <NumberInput
        label={props.name}
        description={props.property.description}
        required={props.required}
        min={props.property.minimum}
        max={props.property.maximum}
        value={typeof props.value === "number" ? props.value : ""}
        onChange={(value) => props.onChange(value)}
      />
    );
  }
  if (props.property.type === "boolean") {
    return (
      <Checkbox
        label={props.name}
        description={props.property.description}
        checked={Boolean(props.value)}
        onChange={(event) => props.onChange(event.currentTarget.checked)}
      />
    );
  }
  return (
    <TextInput
      label={props.name}
      description={props.property.description}
      required={props.required}
      value={typeof props.value === "string" ? props.value : ""}
      onChange={(event) => props.onChange(event.currentTarget.value)}
    />
  );
};

/** Schema-driven form generated from an adapter's manifest config_schema (docs/SPEC.md §5.2). */
export const AddSourceForm: React.FC<AddSourceFormTypes.Props> = (props) => {
  const schema = props.adapter.configSchema as ConfigSchema;
  const [values, setValues] = useState<Record<string, unknown>>(() =>
    addSourceFormUtils.buildInitialValues(schema),
  );
  const [testResult, setTestResult] = useState<string[] | null>(null);

  function handleFieldChange(key: string, value: unknown) {
    setValues((prev) => ({ ...prev, [key]: value }));
    setTestResult(null);
  }

  function handleTestConnection() {
    setTestResult(addSourceFormUtils.validate(schema, values));
  }

  function handleSubmit(event: React.FormEvent) {
    event.preventDefault();
    const errors = addSourceFormUtils.validate(schema, values);
    if (errors.length > 0) {
      setTestResult(errors);
      return;
    }
    props.onSubmit(values);
  }

  const requiredFields = new Set(schema.required ?? []);

  return (
    <form onSubmit={handleSubmit}>
      <Stack gap="sm">
        {Object.entries(schema.properties ?? {}).map(([key, property]) => (
          <ConfigField
            key={key}
            name={key}
            property={property}
            required={requiredFields.has(key)}
            value={values[key]}
            onChange={(value) => handleFieldChange(key, value)}
          />
        ))}
        {testResult ? (
          testResult.length === 0 ? (
            <Alert color="statusOk">Configuration looks valid.</Alert>
          ) : (
            <Alert color="statusCritical">
              {testResult.map((error) => (
                <Typography key={error}>{error}</Typography>
              ))}
            </Alert>
          )
        ) : null}
        <Group justify="flex-end">
          <Button variant="light" onClick={handleTestConnection}>
            Test connection
          </Button>
          <Button type="submit" loading={props.isSubmitting}>
            Add source
          </Button>
        </Group>
      </Stack>
    </form>
  );
};
