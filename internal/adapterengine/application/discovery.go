// discovery.go lists installed adapters by scanning the adapters directory for subdirectories
// containing a manifest.json, per docs/SPEC.md §4 (GET /api/adapters).
package application

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"sre-kit/internal/adapterengine/domain"
)

// InstalledAdapter pairs a discovered adapter's manifest with the directory it was found in.
type InstalledAdapter struct {
	Dir      string
	Manifest domain.Manifest
}

// ListInstalled scans dir for immediate subdirectories containing a manifest.json, parses and
// validates each, and returns the ones that pass. An unreadable dir returns an error; a missing
// dir is treated as "no adapters installed" (empty slice, no error) since it's a normal state on
// first run before any adapter is dropped in.
func ListInstalled(dir string) ([]InstalledAdapter, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("adapterengine: read adapters dir %s: %w", dir, err)
	}

	var installed []InstalledAdapter
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		adapterDir := filepath.Join(dir, entry.Name())
		manifestPath := filepath.Join(adapterDir, "manifest.json")
		raw, err := os.ReadFile(manifestPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("adapterengine: read %s: %w", manifestPath, err)
		}
		var manifest domain.Manifest
		if err := json.Unmarshal(raw, &manifest); err != nil {
			return nil, fmt.Errorf("adapterengine: parse %s: %w", manifestPath, err)
		}
		if err := manifest.Validate(); err != nil {
			return nil, fmt.Errorf("adapterengine: %s: %w", manifestPath, err)
		}
		installed = append(installed, InstalledAdapter{Dir: adapterDir, Manifest: manifest})
	}
	return installed, nil
}
