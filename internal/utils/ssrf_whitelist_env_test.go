package utils

import "testing"

// The environment-only whitelist must follow the environment. It used to be
// parsed once for the life of the process, so whichever test read it first
// decided for every later one and results depended on the order they ran in.
func TestEnvWhitelistFollowsTheEnvironment(t *testing.T) {
	ResetSSRFWhitelistForTest()
	t.Cleanup(ResetSSRFWhitelistForTest)

	t.Setenv("SSRF_WHITELIST", "")
	if err := ValidateURLForSSRF("http://127.0.0.1:8080"); err == nil {
		t.Fatal("loopback accepted with an empty whitelist")
	}

	t.Setenv("SSRF_WHITELIST", "127.0.0.1")
	if err := ValidateURLForSSRF("http://127.0.0.1:8080"); err != nil {
		t.Fatalf("loopback refused although whitelisted: %v", err)
	}

	t.Setenv("SSRF_WHITELIST", "")
	if err := ValidateURLForSSRF("http://127.0.0.1:8080"); err == nil {
		t.Fatal("loopback still accepted after the whitelist was emptied")
	}
}
