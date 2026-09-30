// Bootstrap-time hooks that run after the DI container is built but
// before the HTTP server starts listening. These are deliberately
// best-effort: any failure here only warns and does NOT abort startup.
// The reasoning is that an operator running with a misconfigured env
// var should still be able to bring the server up (and fix the issue
// from the running instance) rather than have a typo brick the deploy.
package main

import (
	"context"
	"os"
	"strings"

	"go.uber.org/dig"

	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	"github.com/magicyuan876/yuheng/internal/utils"
)

// bootstrapEnvVar is the env var that names the email of the user who
// may be promoted to system administrator when the deployment has no
// existing system administrators.
//
// Why an env var (vs a CLI subcommand)?
//   - Zero-friction in docker-compose / k8s deploys: set it once in the
//     manifest and the very first user account that signs up with that
//     email is auto-promoted, with no extra ops step.
//   - Idempotent: if the user is already a system admin, bootstrapping is
//     a no-op.
//   - Safe to leave set: once at least one system admin exists, the env
//     var stops granting privileges. That prevents a UI revoke from being
//     silently undone on the next restart.
const bootstrapEnvVar = "YUHENG_BOOTSTRAP_SYSTEM_ADMIN_EMAIL"

// runStartupBootstrap consults the env and applies any one-shot
// bootstrap actions. Currently it only handles system-admin promotion;
// future bootstrap steps (default model seeding, etc.) can be added
// here as additional dig.Invoke calls.
func runStartupBootstrap(c *dig.Container) {
	ctx := context.Background()

	// Wire the runtime-tunable upload limits (file.max_size_mb /
	// file.video_max_size_mb system settings) into the utils/filesize
	// helpers so every existing call site becomes dynamic without a DI
	// edit. GetInt falls back to the legacy env vars, then the built-in
	// defaults, so unwired deployments behave exactly as before.
	if err := c.Invoke(func(ss interfaces.SystemSettingService) {
		utils.RegisterMaxFileSizeResolvers(
			func() int64 {
				return ss.GetInt(context.Background(), "file.max_size_mb", "MAX_FILE_SIZE_MB", 50)
			},
			func() int64 {
				return ss.GetInt(context.Background(), "file.video_max_size_mb", "MAX_VIDEO_FILE_SIZE_MB", 2048)
			},
		)
	}); err != nil {
		logger.Warnf(ctx, "[bootstrap] failed to resolve SystemSettingService for upload limits: %v", err)
	}

	// Install the platform layer for parser engine configuration. Without it
	// types.ResolveParserEngineOverrides sees no platform default and falls
	// back to the pre-existing ENV < workspace behaviour, so a failure here
	// degrades rather than breaking document parsing.
	if err := c.Invoke(func(resolver interfaces.ParserEngineResolver) {
		if reg, ok := resolver.(interface{ RegisterPlatformLayer() }); ok {
			reg.RegisterPlatformLayer()
		}
	}); err != nil {
		logger.Warnf(ctx, "[bootstrap] failed to install the platform parser engine layer: %v", err)
	}

	// Keys created before migration 000132 are still stored in a
	// recoverable form; seal them into a hash and a hint. Runs on every
	// start; once none are left it is one query over a small table.
	if err := c.Invoke(func(apiKeySvc interfaces.TenantAPIKeyService) {
		if n, err := apiKeySvc.SealStoredKeys(ctx); err != nil {
			logger.Warnf(ctx, "[bootstrap] sealing stored API keys failed: %v", err)
		} else if n > 0 {
			logger.Infof(ctx, "[bootstrap] sealed %d stored API key(s): only their hash and hint are kept", n)
		}
	}); err != nil {
		logger.Warnf(ctx, "[bootstrap] failed to resolve TenantAPIKeyService: %v", err)
	}

	email := strings.TrimSpace(os.Getenv(bootstrapEnvVar))
	if email == "" {
		return
	}
	// dig.Invoke resolves UserService from the container; if user
	// service registration is broken we want to know loudly, but still
	// not abort startup — bootstrap is best-effort.
	if err := c.Invoke(func(userSvc interfaces.UserService) {
		bootstrapSystemAdmin(ctx, userSvc, email)
	}); err != nil {
		logger.Warnf(ctx, "[bootstrap] failed to resolve UserService: %v", err)
	}
}

// bootstrapSystemAdmin promotes the user identified by `email` to system
// administrator only when the deployment currently has no system admins.
// The function is idempotent and non-fatal — it warns and returns on
// every error path.
//
// The bootstrap intentionally does NOT create a user when the email is
// not yet registered: account creation is a workflow with side effects
// (password hashing, tenant assignment, audit) that we don't want to
// short-circuit. Operators should sign up normally first, then set the
// env var on the next restart.
func bootstrapSystemAdmin(ctx context.Context, userSvc interfaces.UserService, email string) {
	user, err := userSvc.GetUserByEmail(ctx, email)
	if err != nil {
		// "not found" surfaces as an error in this codebase; treat it
		// gently — operators commonly set the var before the user has
		// signed up. The next restart after registration will succeed.
		logger.Warnf(ctx,
			"[bootstrap] %s=%s: user lookup failed (have they signed up yet?): %v",
			bootstrapEnvVar, email, err)
		return
	}
	if user == nil {
		logger.Warnf(ctx,
			"[bootstrap] %s=%s: no matching user (will retry on next restart)",
			bootstrapEnvVar, email)
		return
	}
	if user.IsSystemAdmin {
		logger.Infof(ctx,
			"[bootstrap] %s=%s: user %s is already a system admin (no-op)",
			bootstrapEnvVar, email, user.ID)
		return
	}
	_, total, err := userSvc.ListSystemAdmins(ctx, 0, 1)
	if err != nil {
		logger.Warnf(ctx,
			"[bootstrap] %s=%s: cannot verify existing system admins, skipping promotion: %v",
			bootstrapEnvVar, email, err)
		return
	}
	if total > 0 {
		logger.Infof(ctx,
			"[bootstrap] %s=%s: %d system admin(s) already exist; not promoting user %s",
			bootstrapEnvVar, email, total, user.ID)
		return
	}
	user.IsSystemAdmin = true
	if err := userSvc.UpdateUser(ctx, user); err != nil {
		logger.Warnf(ctx,
			"[bootstrap] %s=%s: failed to promote user %s: %v",
			bootstrapEnvVar, email, user.ID, err)
		return
	}
	logger.Infof(ctx,
		"[bootstrap] promoted user %s (%s) to system admin via %s",
		user.ID, email, bootstrapEnvVar)
}
