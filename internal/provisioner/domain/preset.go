package domain

import "fmt"

// BootstrapStepType is how a preset's bootstrap step creates the deployed tool's first admin
// account, per docs/SPEC.md §12.3: Beszel's is one SSH-run CLI command, Umami's is an HTTP call
// sequence against the freshly deployed container — the two v1 presets have materially different
// bootstrap shapes, so both step types are supported from the start.
type BootstrapStepType string

const (
	BootstrapSSHCommand BootstrapStepType = "ssh_command"
	BootstrapHTTPCall   BootstrapStepType = "http_call"
)

// BootstrapStep is one step of a preset's bootstrap sequence. Templated fields are Go
// text/template strings rendered against the workflow's TemplateData (application package).
//
// ssh_command: Command is run over the host's SSH connection.
// http_call: Method/PathTemplate/BodyTemplate target the freshly deployed tool's own HTTP API
// (reached directly, not over SSH). AuthFromField, if set, names a value captured by an earlier
// step (via CaptureField/CaptureAs) to send as "Authorization: Bearer <value>". CaptureField, if
// set, names a top-level field in the JSON response to capture under CaptureAs for later steps.
type BootstrapStep struct {
	Type BootstrapStepType `json:"type"`

	Command string `json:"command,omitempty"`

	Method       string `json:"method,omitempty"`
	PathTemplate string `json:"path_template,omitempty"`
	BodyTemplate string `json:"body_template,omitempty"`

	AuthFromField string `json:"auth_from_field,omitempty"`
	CaptureField  string `json:"capture_field,omitempty"`
	CaptureAs     string `json:"capture_as,omitempty"`
}

// Manifest is a preset's manifest.json, per docs/SPEC.md §12.3.
type Manifest struct {
	Name                         string            `json:"name"`
	Version                      string            `json:"version"`
	ProducesAdapter              string            `json:"produces_adapter"`
	ProducesSourceConfigTemplate map[string]string `json:"produces_source_config_template"`
	BaseURLTemplate              string            `json:"base_url_template"`
}

// Preset pairs a manifest with its docker-compose template and bootstrap sequence — the three
// files under presets/<name>/ (manifest.json, docker-compose.yml.tmpl, bootstrap.json).
type Preset struct {
	Manifest        Manifest
	ComposeTemplate string
	Bootstrap       []BootstrapStep
}

// Validate checks the structural requirements a preset must satisfy to be usable.
func (p Preset) Validate() error {
	if p.Manifest.Name == "" {
		return fmt.Errorf("provisioner: preset manifest: name is required")
	}
	if p.Manifest.ProducesAdapter == "" {
		return fmt.Errorf("provisioner: preset %q: produces_adapter is required", p.Manifest.Name)
	}
	if p.ComposeTemplate == "" {
		return fmt.Errorf("provisioner: preset %q: docker-compose.yml.tmpl is empty", p.Manifest.Name)
	}
	if len(p.Bootstrap) == 0 {
		return fmt.Errorf("provisioner: preset %q: bootstrap.json declares no steps", p.Manifest.Name)
	}
	for i, step := range p.Bootstrap {
		switch step.Type {
		case BootstrapSSHCommand:
			if step.Command == "" {
				return fmt.Errorf("provisioner: preset %q: bootstrap step %d: command is required for ssh_command", p.Manifest.Name, i)
			}
		case BootstrapHTTPCall:
			if step.Method == "" || step.PathTemplate == "" {
				return fmt.Errorf("provisioner: preset %q: bootstrap step %d: method and path_template are required for http_call", p.Manifest.Name, i)
			}
		default:
			return fmt.Errorf("provisioner: preset %q: bootstrap step %d: unknown type %q", p.Manifest.Name, i, step.Type)
		}
	}
	return nil
}
