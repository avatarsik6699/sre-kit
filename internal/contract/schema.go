// Package contract embeds the repo-root contract.schema.json (via a symlink into this package
// directory, since go:embed cannot reach outside its own directory tree) and validates NDJSON
// lines emitted by adapters against it, per docs/SPEC.md §4.
package contract

import (
	"bytes"
	_ "embed"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed contract.schema.json
var schemaJSON []byte

const schemaURL = "https://sre-kit.local/contract.schema.json"

var compiledSchema *jsonschema.Schema

func init() {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaJSON))
	if err != nil {
		panic(fmt.Errorf("contract: parse embedded schema: %w", err))
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(schemaURL, doc); err != nil {
		panic(fmt.Errorf("contract: register embedded schema: %w", err))
	}
	compiledSchema, err = compiler.Compile(schemaURL)
	if err != nil {
		panic(fmt.Errorf("contract: compile embedded schema: %w", err))
	}
}

// ValidateLine parses a single NDJSON line and validates it against contract.schema.json. It
// returns an error describing why the line is invalid (malformed JSON or a schema violation); a
// nil return means the line is a well-formed metric/check/event/alert per the current contract
// version.
func ValidateLine(line []byte) error {
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(line))
	if err != nil {
		return fmt.Errorf("contract: invalid JSON: %w", err)
	}
	if err := compiledSchema.Validate(value); err != nil {
		return fmt.Errorf("contract: schema violation: %w", err)
	}
	return nil
}
