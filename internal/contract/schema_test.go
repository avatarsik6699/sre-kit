package contract

import "testing"

func TestValidateLine_ValidMetric(t *testing.T) {
	line := []byte(`{"type":"metric","source_id":"src-1","name":"cpu.usage_percent","timestamp":"2026-08-13T12:00:00Z","value":42.5,"labels":{"host":"vps-1"}}`)
	if err := ValidateLine(line); err != nil {
		t.Fatalf("ValidateLine(valid metric) = %v, want nil", err)
	}
}

func TestValidateLine_ValidCheck(t *testing.T) {
	line := []byte(`{"type":"check","source_id":"src-1","name":"tls-expiry","timestamp":"2026-08-13T12:00:00Z","status":"ok"}`)
	if err := ValidateLine(line); err != nil {
		t.Fatalf("ValidateLine(valid check) = %v, want nil", err)
	}
}

func TestValidateLine_ValidEvent(t *testing.T) {
	line := []byte(`{"type":"event","source_id":"src-1","timestamp":"2026-08-13T12:00:00Z","level":"warn","message":"fail2ban banned 1.2.3.4"}`)
	if err := ValidateLine(line); err != nil {
		t.Fatalf("ValidateLine(valid event) = %v, want nil", err)
	}
}

func TestValidateLine_MissingRequiredField(t *testing.T) {
	line := []byte(`{"type":"metric","source_id":"src-1","timestamp":"2026-08-13T12:00:00Z","value":42.5}`)
	if err := ValidateLine(line); err == nil {
		t.Fatal("ValidateLine(missing name) = nil, want error")
	}
}

func TestValidateLine_UnknownType(t *testing.T) {
	line := []byte(`{"type":"bogus","source_id":"src-1"}`)
	if err := ValidateLine(line); err == nil {
		t.Fatal("ValidateLine(unknown type) = nil, want error")
	}
}

func TestValidateLine_MalformedJSON(t *testing.T) {
	if err := ValidateLine([]byte(`not json`)); err == nil {
		t.Fatal("ValidateLine(malformed JSON) = nil, want error")
	}
}

func TestValidateLine_AdapterCannotEmitAlert(t *testing.T) {
	// alert is core-generated only (SPEC §4), but the schema still accepts a well-formed one on
	// the wire (e.g. over the WS stream) — enforcement that adapters don't emit it belongs to the
	// adapter runner, not the schema.
	line := []byte(`{"type":"alert","source_id":"src-1","severity":"critical","message":"down","created_at":"2026-08-13T12:00:00Z"}`)
	if err := ValidateLine(line); err != nil {
		t.Fatalf("ValidateLine(valid alert) = %v, want nil", err)
	}
}
