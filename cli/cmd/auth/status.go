package auth

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/magicyuan876/yuheng/cli/internal/cmdutil"
	"github.com/magicyuan876/yuheng/cli/internal/iostreams"
	sdk "github.com/magicyuan876/yuheng/client"
)

// authStatusFields enumerates the fields surfaced for `--format json` discovery
// on `auth status`. Single-resource shape: filter applies to data itself.
var authStatusFields = []string{
	"profile", "host", "auth_source", "user_id", "username", "email", "is_active",
	"can_access_all_tenants", "tenant_id", "tenant_name",
}

// StatusService is the narrow SDK surface auth status depends on.
type StatusService interface {
	GetCurrentUser(ctx context.Context) (*sdk.CurrentUserResponse, error)
}

// statusResult is the typed payload emitted by `--format json`. Mirrors the
// SDK AuthUser + AuthTenant projection so agents can branch on
// can_access_all_tenants (cross-tenant admin) and is_active (disabled
// account) without a second round-trip. tenant_id / tenant_name describe the
// tenant the credential is scoped to and are absent for a user who belongs
// to no tenant yet.
type statusResult struct {
	Profile             string `json:"profile"`
	Host                string `json:"host,omitempty"`
	AuthSource          string `json:"auth_source,omitempty"`
	UserID              string `json:"user_id,omitempty"`
	Username            string `json:"username,omitempty"`
	Email               string `json:"email,omitempty"`
	IsActive            bool   `json:"is_active,omitempty"`
	CanAccessAllTenants bool   `json:"can_access_all_tenants,omitempty"`
	TenantID            uint64 `json:"tenant_id,omitempty"`
	TenantName          string `json:"tenant_name,omitempty"`
}

// NewCmdStatus builds the `yuheng auth status` command.
func NewCmdStatus(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show the active profile, principal, and token state",
		Long: `Live-check the active credential by calling /auth/me. Reports the user
and tenant the server resolves the credential to.

Exits with auth.unauthenticated when the token is invalid or missing - run
` + "`yuheng auth login`" + ` (or ` + "`auth refresh`" + ` for JWT profiles) to recover.
For JWT profiles the SDK transparently refreshes on 401, so this command
usually only surfaces a hard auth failure.`,
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			fopts, err := cmdutil.CheckFormatFlag(c)
			if err != nil {
				return err
			}
			fopts.ResolveDefault(iostreams.IO.IsStdoutTTY())
			cli, err := f.Client()
			if err != nil {
				return err
			}
			return runStatus(c.Context(), fopts, f, cli)
		},
	}
	cmdutil.AddFormatFlag(cmd, authStatusFields...)
	cmdutil.SetAgentHelp(cmd, cmdutil.AgentHelp{
		UsedFor:  "show the active profile, the authenticated principal, and token state",
		Examples: []string{"yuheng auth status", "yuheng auth status --jq .data.tenant_id"},
		Output:   "envelope.data is {profile, host, auth_source, user_id, username, email, tenant_id, tenant_name, ...}; auth_source is 'profile + keyring' or a stateless env credential; exit 3 if unauthenticated",
	})
	return cmd
}

func runStatus(ctx context.Context, fopts *cmdutil.FormatOptions, f *cmdutil.Factory, svc StatusService) error {
	if svc == nil {
		return cmdutil.NewError(cmdutil.CodeAuthUnauthenticated, "no SDK client available; run `yuheng auth login`")
	}
	resp, err := svc.GetCurrentUser(ctx)
	if err != nil {
		return cmdutil.WrapHTTP(err, "fetch current user")
	}
	user := resp.Data.User
	tenant := resp.Data.Tenant

	cfg, err := f.Config()
	if err != nil {
		return err
	}

	// Effective host + auth source. Stateless env credentials
	// (YUHENG_TOKEN/YUHENG_API_KEY) override the profile + keyring for the
	// client, so report the host they authenticate against (YUHENG_HOST) and
	// that they are in effect — not the bypassed config profile's host.
	host := ""
	if c, ok := cfg.Profiles[cfg.CurrentProfile]; ok {
		host = c.Host
	}
	authSource := "profile + keyring"
	if active, kind := cmdutil.EnvCredential(); active {
		authSource = kind + " env (stateless)"
		if h := strings.TrimSpace(os.Getenv("YUHENG_HOST")); h != "" {
			host = h
		}
	}

	if fopts.WantsJSON() {
		result := statusResult{Profile: cfg.CurrentProfile, Host: host, AuthSource: authSource}
		if user != nil {
			result.UserID = user.ID
			result.Username = user.Username
			result.Email = user.Email
			result.IsActive = user.IsActive
			result.CanAccessAllTenants = user.CanAccessAllTenants
		}
		if tenant != nil {
			result.TenantID = tenant.ID
			result.TenantName = tenant.Name
		}
		return fopts.Emit(iostreams.IO.Out, result, nil)
	}

	fmt.Fprintf(iostreams.IO.Out, "profile:     %s\n", cfg.CurrentProfile)
	fmt.Fprintf(iostreams.IO.Out, "auth_source: %s\n", authSource)
	fmt.Fprintf(iostreams.IO.Out, "host:        %s\n", host)
	if user != nil {
		fmt.Fprintf(iostreams.IO.Out, "user:    %s (%s)\n", user.Email, user.ID)
		if tenant != nil {
			fmt.Fprintf(iostreams.IO.Out, "tenant:  %d (%s)\n", tenant.ID, tenant.Name)
		} else {
			fmt.Fprintln(iostreams.IO.Out, "tenant:  none (ask an administrator to add you to a workspace)")
		}
	}
	return nil
}
