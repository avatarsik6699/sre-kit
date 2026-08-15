package application_test

import (
	"testing"

	"sre-kit/internal/provisioner/application"
)

// TestRealPresetsLoadAndValidate guards the actual presets/ directory shipped with the repo
// (docs/SPEC.md §12.3: beszel, umami) against typos/schema drift — LoadPreset is exactly what the
// workflow calls at runtime.
func TestRealPresetsLoadAndValidate(t *testing.T) {
	for _, name := range []string{"beszel", "umami"} {
		t.Run(name, func(t *testing.T) {
			preset, err := application.LoadPreset("../../../presets", name)
			if err != nil {
				t.Fatalf("LoadPreset(%q): %v", name, err)
			}
			if preset.Manifest.ProducesAdapter == "" {
				t.Fatal("produces_adapter is empty")
			}
			if len(preset.Bootstrap) == 0 {
				t.Fatal("bootstrap has no steps")
			}
		})
	}
}
