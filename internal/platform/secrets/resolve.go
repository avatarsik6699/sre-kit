package secrets

import (
	"encoding/json"
	"fmt"
)

// schemaProperty is the subset of a JSON Schema property this package cares about: whether an
// adapter manifest's config_schema (docs/SPEC.md §4) marks a field as holding a secret_ref rather
// than a plain value.
type schemaProperty struct {
	Format string `json:"format"`
}

type configSchemaDoc struct {
	Properties map[string]schemaProperty `json:"properties"`
}

// ResolveConfig returns configJSON with every field configSchema marks `"format": "secret"`
// replaced in place: that field's value (a secret_ref, as produced by Store.Put) is swapped for
// the plaintext secret held in store. Per docs/SPEC.md §3, sources.config_json only ever persists
// the ref — this is the one place, at adapter-spawn time, where the ref is resolved back to a
// real value. The result is meant to be written only to an adapter subprocess's stdin, never
// persisted or logged.
func ResolveConfig(store *Store, configSchema json.RawMessage, configJSON string) ([]byte, error) {
	var schema configSchemaDoc
	if len(configSchema) > 0 {
		if err := json.Unmarshal(configSchema, &schema); err != nil {
			return nil, fmt.Errorf("secrets: parse config_schema: %w", err)
		}
	}

	var secretFields []string
	for name, prop := range schema.Properties {
		if prop.Format == "secret" {
			secretFields = append(secretFields, name)
		}
	}
	if len(secretFields) == 0 {
		return []byte(configJSON), nil
	}

	var config map[string]json.RawMessage
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return nil, fmt.Errorf("secrets: parse config: %w", err)
	}

	for _, field := range secretFields {
		raw, ok := config[field]
		if !ok {
			continue
		}
		var ref string
		if err := json.Unmarshal(raw, &ref); err != nil {
			return nil, fmt.Errorf("secrets: field %q: expected a string secret_ref: %w", field, err)
		}
		value, err := store.Get(ref)
		if err != nil {
			return nil, fmt.Errorf("secrets: resolve field %q: %w", field, err)
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("secrets: encode resolved field %q: %w", field, err)
		}
		config[field] = encoded
	}

	resolved, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("secrets: encode resolved config: %w", err)
	}
	return resolved, nil
}
