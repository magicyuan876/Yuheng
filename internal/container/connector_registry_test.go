package container

import "testing"

// The built-in connectors and their UI metadata match one for one, so
// GET /datasource/types offers exactly the connectors that can sync.
func TestInitConnectorRegistryMatchesMetadata(t *testing.T) {
	registry, err := initConnectorRegistry()
	if err != nil {
		t.Fatalf("initConnectorRegistry: %v", err)
	}
	if got, want := len(registry.Available()), len(registry.List()); got != want {
		t.Fatalf("Available() lists %d connectors, %d are registered", got, want)
	}
}
