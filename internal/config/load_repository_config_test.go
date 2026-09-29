package config

import (
	"os"
	"testing"

	"github.com/spf13/viper"
)

// The configuration file that ships in the repository must load and validate
// under every value of the registration switch an operator can set. A mode the
// code accepts but the validator rejects (which is what "auto" once was) keeps
// the server from starting at all, and no unit test of either half sees it.
func TestRepositoryConfigLoadsForEveryRegistrationSetting(t *testing.T) {
	for _, disable := range []string{"", "auto", "true", "false"} {
		t.Run("DISABLE_REGISTRATION="+disable, func(t *testing.T) {
			t.Setenv("DISABLE_REGISTRATION", disable)

			wd, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Chdir("../.."); err != nil { // the repository root, where ./config lives
				t.Fatal(err)
			}
			t.Cleanup(func() {
				viper.Reset()
				_ = os.Chdir(wd)
			})
			viper.Reset()

			cfg, err := LoadConfig()
			if err != nil {
				t.Fatalf("LoadConfig: %v", err)
			}
			if cfg.Auth == nil || cfg.Auth.RegistrationMode == "" {
				t.Fatalf("no registration mode after loading: %+v", cfg.Auth)
			}
		})
	}
}

func TestValidateConfigAcceptsEveryRegistrationModeAndRejectsATypo(t *testing.T) {
	for _, mode := range []string{"", AuthRegistrationModeAuto, AuthRegistrationModeSelfServe, AuthRegistrationModeInviteOnly} {
		cfg := &Config{Auth: &AuthConfig{RegistrationMode: mode}, Server: &ServerConfig{Port: 8080}}
		if err := ValidateConfig(cfg); err != nil {
			t.Errorf("mode %q rejected: %v", mode, err)
		}
	}
	cfg := &Config{Auth: &AuthConfig{RegistrationMode: "open"}, Server: &ServerConfig{Port: 8080}}
	if err := ValidateConfig(cfg); err == nil {
		t.Error("a mode that does not exist was accepted")
	}
}
