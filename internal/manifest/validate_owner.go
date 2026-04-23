package manifest

import (
	"fmt"

	"github.com/enekos/inguma/internal/namespace"
)

// ValidateWithOwner performs base Validate plus namespace consistency:
// the manifest's Name must either be a bare slug or a @<registryOwner>/<slug>.
func ValidateWithOwner(m *Tool, registryOwner string) error {
	if err := Validate(m); err != nil {
		return err
	}
	n, err := namespace.Parse(m.Name)
	if err != nil {
		return fmt.Errorf("manifest: name: %w", err)
	}
	if !n.IsBare && n.Owner != registryOwner {
		return fmt.Errorf("manifest: name owner %q does not match registry owner %q", n.Owner, registryOwner)
	}
	return nil
}
