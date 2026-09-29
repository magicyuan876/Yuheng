package handler

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"testing"
)

func TestDeploymentCapabilityKeysMatchFrontend(t *testing.T) {
	frontendKeys, err := readFrontendDeploymentCapabilityKeys()
	if err != nil {
		t.Fatalf("read frontend capability keys: %v", err)
	}

	if !slices.Equal(DeploymentCapabilityKeys, frontendKeys) {
		t.Fatalf("backend keys = %#v, frontend keys = %#v", DeploymentCapabilityKeys, frontendKeys)
	}
}

func TestBuildDeploymentCapabilitiesIncludesAllKeys(t *testing.T) {
	result := BuildDeploymentCapabilities(DeploymentFeatureAvailability{
		Organizations: true,
		WebSearch:     true,
		VectorStore:   true,
		Storage:       true,
		Docs:          true,
	})

	for _, key := range DeploymentCapabilityKeys {
		if _, ok := result.Capabilities[key]; !ok {
			t.Fatalf("missing capability key %q", key)
		}
	}
}

func readFrontendDeploymentCapabilityKeys() ([]string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return nil, os.ErrInvalid
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	frontendPath := filepath.Join(repoRoot, "frontend", "src", "config", "deploymentCapabilities.ts")
	content, err := os.ReadFile(frontendPath)
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile(`(?s)export const DEPLOYMENT_CAPABILITY_KEYS = \[(.*?)\]`)
	match := re.FindSubmatch(content)
	if len(match) < 2 {
		return nil, os.ErrInvalid
	}

	// Take the string literals themselves rather than trimming lines. Trimming
	// broke twice on formatting alone: a Windows checkout's "\r" line endings
	// kept the trailing comma, and when Prettier moved the file from single to
	// double quotes every key came back still wrapped in quotes. A literal is
	// either quote style; line breaks, commas and the `as const` around it
	// no longer matter.
	literal := regexp.MustCompile(`'([^']*)'|"([^"]*)"`)
	var keys []string
	for _, m := range literal.FindAllSubmatch(match[1], -1) {
		keys = append(keys, string(m[1])+string(m[2]))
	}
	return keys, nil
}
