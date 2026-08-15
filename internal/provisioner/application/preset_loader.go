// preset_loader.go lists/loads installed presets by scanning the presets directory for
// subdirectories containing a manifest.json + docker-compose.yml.tmpl + bootstrap.json, per
// docs/SPEC.md §12.3 — mirrors internal/adapterengine/application/discovery.go's ListInstalled,
// generalized from adapters to presets.
package application

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"sre-kit/internal/provisioner/domain"
)

type bootstrapFile struct {
	Steps []domain.BootstrapStep `json:"steps"`
}

// ListInstalledPresets scans dir for immediate subdirectories containing a manifest.json, parses
// and validates each preset, and returns the ones that pass. A missing dir is treated as "no
// presets installed" (empty slice, no error) — same tolerance ListInstalled gives adapters.
func ListInstalledPresets(dir string) ([]domain.Preset, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("provisioner: read presets dir %s: %w", dir, err)
	}

	var presets []domain.Preset
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		preset, err := loadPreset(dir, entry.Name())
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		presets = append(presets, preset)
	}
	return presets, nil
}

// loadPreset loads and validates the single preset named name from dir/name/.
func loadPreset(dir, name string) (domain.Preset, error) {
	presetDir := filepath.Join(dir, name)

	manifestRaw, err := os.ReadFile(filepath.Join(presetDir, "manifest.json"))
	if err != nil {
		return domain.Preset{}, err
	}
	var manifest domain.Manifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		return domain.Preset{}, fmt.Errorf("provisioner: parse %s/manifest.json: %w", presetDir, err)
	}

	composeRaw, err := os.ReadFile(filepath.Join(presetDir, "docker-compose.yml.tmpl"))
	if err != nil {
		return domain.Preset{}, fmt.Errorf("provisioner: read %s/docker-compose.yml.tmpl: %w", presetDir, err)
	}

	bootstrapRaw, err := os.ReadFile(filepath.Join(presetDir, "bootstrap.json"))
	if err != nil {
		return domain.Preset{}, fmt.Errorf("provisioner: read %s/bootstrap.json: %w", presetDir, err)
	}
	var bootstrap bootstrapFile
	if err := json.Unmarshal(bootstrapRaw, &bootstrap); err != nil {
		return domain.Preset{}, fmt.Errorf("provisioner: parse %s/bootstrap.json: %w", presetDir, err)
	}

	preset := domain.Preset{
		Manifest:        manifest,
		ComposeTemplate: string(composeRaw),
		Bootstrap:       bootstrap.Steps,
	}
	if err := preset.Validate(); err != nil {
		return domain.Preset{}, err
	}
	return preset, nil
}

// LoadPreset loads the preset named name from dir — exported for the workflow Service, which
// operates on one named preset per run rather than the full installed list.
func LoadPreset(dir, name string) (domain.Preset, error) {
	preset, err := loadPreset(dir, name)
	if os.IsNotExist(err) {
		return domain.Preset{}, fmt.Errorf("provisioner: preset %q not installed", name)
	}
	return preset, err
}
