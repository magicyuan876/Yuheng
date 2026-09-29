package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/gin-gonic/gin"

	"github.com/magicyuan876/yuheng/internal/logger"
)

// SilenceGinRouteSpam mutes Gin's per-route "[GIN-debug] METHOD path -->
// handler" lines that flood the startup log with one entry per registered
// route (we have ~150 of them). DebugMode features (panic recovery
// stacktraces, runtime warnings) are preserved.
//
// In place of the per-route spam, prints a single summary line at the
// end of router build with the route count.
//
// Call once at the start of main(), before container.BuildContainer.
func SilenceGinRouteSpam() {
	var count int64
	gin.DebugPrintRouteFunc = func(httpMethod, absolutePath, handlerName string, nuHandlers int) {
		atomic.AddInt64(&count, 1)
	}
	// gin.DebugPrintFunc handles non-route debug lines (e.g. "Listening
	// and serving HTTP"). Route them through the structured logger so
	// they share the same format as the rest of startup.
	gin.DebugPrintFunc = func(format string, values ...interface{}) {
		msg := strings.TrimRight(fmt.Sprintf(format, values...), "\n")
		// Drop the redundant "Creating an Engine instance with the Logger
		// and Recovery middleware already attached" — purely informational.
		if strings.Contains(msg, "Creating an Engine instance") {
			return
		}
		logger.Info(context.Background(), "[gin] "+msg)
	}
	// Expose a way to print the suppressed count once routes are wired up.
	ginRouteCount = &count
}

// ginRouteCount is set by SilenceGinRouteSpam and read by LogGinRouteCount
// so callers can print a one-line summary after router build.
var ginRouteCount *int64

// LogGinRouteCount writes a single summary line for the number of routes
// Gin registered. Safe to call even if SilenceGinRouteSpam wasn't called
// (it just becomes a no-op).
func LogGinRouteCount(ctx context.Context) {
	if ginRouteCount == nil {
		return
	}
	logger.Infof(ctx, "[gin] registered %d routes", atomic.LoadInt64(ginRouteCount))
}

// envVarSpec describes one env var to surface in the startup banner.
type envVarSpec struct {
	name      string
	sensitive bool // if true, only presence/length is printed, never value
}

// startupEnvVars lists the env vars whose presence is most worth
// surfacing at boot — primarily security-sensitive ones whose silent
// absence has caused real incidents (SYSTEM_AES_KEY rotation, JWT secret
// drift) plus the basic DB / storage selectors that change behaviour
// significantly.
//
// Keep the list short — this banner exists to answer "is the config I
// expect actually loaded?" at a glance, not to dump the entire env.
var startupEnvVars = []envVarSpec{
	// Security
	{name: "SYSTEM_AES_KEY", sensitive: true},
	{name: "JWT_SECRET", sensitive: true},
	{name: "GRPC_AUTH_TOKEN", sensitive: true},
	// Runtime
	{name: "GIN_MODE"},
	{name: "AUTO_MIGRATE"},
	// Database
	{name: "DB_DRIVER"},
	{name: "DB_HOST"},
	{name: "DB_PORT"},
	{name: "DB_USER"},
	{name: "DB_NAME"},
	{name: "DB_PASSWORD", sensitive: true},
	// Cache / queue
	{name: "REDIS_ADDR"},
	{name: "REDIS_PASSWORD", sensitive: true},
	{name: "REDIS_USE_TLS"},
	{name: "REDIS_TLS_SERVER_NAME"},
	// Object storage
	{name: "STORAGE_TYPE"},
	{name: "S3_ENDPOINT"},
	{name: "S3_BUCKET_NAME"},
	{name: "S3_SECRET_KEY", sensitive: true},
	// External services
	{name: "DOCREADER_ADDR"},
	{name: "RETRIEVE_DRIVER"},
}

// LogStartupEnv prints a single banner block summarising the curated set
// of env vars in startupEnvVars. Sensitive values are reported as
// "set (N chars)" — the goal is to give the operator confidence the
// config they expected actually landed, without echoing secrets to logs.
//
// Output format (one line per var to stay grep-able):
//
//	[startup-env] SYSTEM_AES_KEY=set (32 chars)
//	[startup-env] JWT_SECRET=<unset>
//	[startup-env] DB_DRIVER=postgres
//
// Finally the deployment secrets are validated and the process exits when
// they are unsafe (see EnforceSecrets).
func LogStartupEnv(ctx context.Context) {
	// Sort by name for deterministic output.
	specs := make([]envVarSpec, len(startupEnvVars))
	copy(specs, startupEnvVars)
	sort.Slice(specs, func(i, j int) bool { return specs[i].name < specs[j].name })

	logger.Info(ctx, "[startup-env] resolved environment:")
	for _, s := range specs {
		val := os.Getenv(s.name)
		logger.Infof(ctx, "[startup-env]   %s=%s", s.name, formatEnvValue(s, val))
	}

	// Secrets that are missing, weak or still the published example are no
	// longer a warning: outside the explicit developer opt-out they stop the
	// server from starting (see EnforceSecrets).
	enforceSecretsOrExit(ctx)

	if strings.EqualFold(strings.TrimSpace(os.Getenv("REDIS_TLS_INSECURE_SKIP_VERIFY")), "true") {
		logger.Warn(ctx,
			"[startup-env] REDIS_TLS_INSECURE_SKIP_VERIFY=true — Redis TLS certificate verification is DISABLED; do not use in production")
	}
}

func formatEnvValue(s envVarSpec, val string) string {
	if val == "" {
		return "<unset>"
	}
	if s.sensitive {
		return fmt.Sprintf("set (%d chars)", len(val))
	}
	return val
}

// InsecureDevEnv is the explicit developer opt-out from secret enforcement.
// It exists so a laptop can run the stack straight from .env.example; it must
// never be set on a deployment anyone else can reach.
const InsecureDevEnv = "YUHENG_INSECURE_DEV"

// Published example values. They are in .env.example / documentation, so they
// are known to everyone who has read the repository.
const (
	exampleJWTSecret   = "yuheng-jwt-secret"
	exampleAESKey      = "yuheng-system-aes-key-32bytes!!"
	exampleGRPCToken   = "your-secret-token-at-least-16-bytes"
	minJWTSecretLength = 32
	aesKeyLength       = 32
	minGRPCTokenLength = 16
)

// secretProblem is one unsafe secret, worded for an operator.
type secretProblem struct {
	name   string
	reason string
	why    string
	how    string
}

func (p secretProblem) String() string {
	return fmt.Sprintf("  %s\n    problem: %s\n    why:     %s\n    fix:     %s", p.name, p.reason, p.why, p.how)
}

// secretProblems returns every unsafe secret in the environment given by
// getenv, or nil when all are acceptable. It does not consult the developer
// opt-out; ValidateSecrets does.
func secretProblems(getenv func(string) string) []secretProblem {
	var out []secretProblem
	add := func(name, reason, why, how string) {
		out = append(out, secretProblem{name: name, reason: reason, why: why, how: how})
	}
	const example = "it is still the example value published in .env.example"

	jwtWhy := "it signs every session token; whoever knows it can mint a valid login for any user, " +
		"including administrators"
	jwtHow := "generate one and set it in .env:  JWT_SECRET=$(openssl rand -hex 32)"
	switch jwt := strings.TrimSpace(getenv("JWT_SECRET")); {
	case jwt == "":
		add("JWT_SECRET", "it is not set (a random one would be generated per start, logging everyone out on "+
			"every restart and breaking multi-instance deployments)", jwtWhy, jwtHow)
	case jwt == exampleJWTSecret:
		add("JWT_SECRET", example, jwtWhy, jwtHow)
	case len(jwt) < minJWTSecretLength:
		add("JWT_SECRET", fmt.Sprintf("it is %d characters long; at least %d are required",
			len(jwt), minJWTSecretLength), jwtWhy, jwtHow)
	}

	aesWhy := "it encrypts model API keys, MCP and data-source credentials and other secrets stored in the " +
		"database; without a valid key they would be stored as plaintext, and with a public one anyone can decrypt them"
	aesHow := "generate one and set it in .env (back it up: losing it makes stored secrets unreadable):  " +
		"SYSTEM_AES_KEY=$(openssl rand -hex 16)   # exactly 32 bytes"
	switch aes := getenv("SYSTEM_AES_KEY"); {
	case aes == "":
		add("SYSTEM_AES_KEY", "it is not set, so encryption at rest would be silently disabled", aesWhy, aesHow)
	case aes == exampleAESKey:
		add("SYSTEM_AES_KEY", example, aesWhy, aesHow)
	case len(aes) != aesKeyLength:
		add("SYSTEM_AES_KEY", fmt.Sprintf("it is %d bytes long; AES-256 needs exactly %d, and any other length "+
			"silently disables encryption", len(aes), aesKeyLength), aesWhy, aesHow)
	}

	// GRPC_AUTH_TOKEN authenticates the docreader link. Unset is a legitimate
	// choice on a private network (docreader then runs without auth and says
	// so), so only a set-but-known or set-but-weak value is an error.
	grpcWhy := "it authenticates calls to the document parser; a known or guessable token is no authentication"
	grpcHow := "generate one and set it for BOTH app and docreader in .env:  GRPC_AUTH_TOKEN=$(openssl rand -hex 24)"
	switch tok := getenv("GRPC_AUTH_TOKEN"); {
	case tok == "":
	case tok == exampleGRPCToken:
		add("GRPC_AUTH_TOKEN", example, grpcWhy, grpcHow)
	case len(tok) < minGRPCTokenLength:
		add("GRPC_AUTH_TOKEN", fmt.Sprintf("it is %d characters long; at least %d are required",
			len(tok), minGRPCTokenLength), grpcWhy, grpcHow)
	}
	return out
}

// ValidateSecrets checks the deployment secrets and returns a multi-line error
// naming each unsafe one, why it matters and how to generate a good value.
// With YUHENG_INSECURE_DEV=true it returns nil (EnforceSecrets logs the
// problems it is waving through).
func ValidateSecrets(getenv func(string) string) error {
	problems := secretProblems(getenv)
	if len(problems) == 0 || insecureDevEnabled(getenv) {
		return nil
	}
	return errors.New(formatsecretProblems(problems))
}

func insecureDevEnabled(getenv func(string) string) bool {
	return strings.EqualFold(strings.TrimSpace(getenv(InsecureDevEnv)), "true")
}

func formatsecretProblems(problems []secretProblem) string {
	var b strings.Builder
	b.WriteString("refusing to start: the deployment secrets are missing or unsafe.\n\n")
	for _, p := range problems {
		b.WriteString(p.String())
		b.WriteString("\n\n")
	}
	b.WriteString("For a throwaway local run only, " + InsecureDevEnv + "=true skips this check " +
		"(secrets then stay weak and encryption at rest may be off).")
	return b.String()
}

// EnforceSecrets validates the secrets from the process environment. It
// returns the multi-line error from ValidateSecrets, or nil; when the developer
// opt-out is active it logs each problem loudly and returns nil.
func EnforceSecrets(ctx context.Context) error {
	getenv := os.Getenv
	if err := ValidateSecrets(getenv); err != nil {
		return err
	}
	if insecureDevEnabled(getenv) {
		for _, p := range secretProblems(getenv) {
			logger.Warnf(ctx, "[startup-env] %s=true: continuing although %s: %s. NEVER run like this in production.",
				InsecureDevEnv, p.name, p.reason)
		}
	}
	return nil
}

// exitFunc is os.Exit, replaceable in tests.
var exitFunc = os.Exit

// enforceSecretsOrExit stops the process on unsafe secrets. It is called from
// LogStartupEnv so that the check runs on every start without depending on the
// caller remembering it.
func enforceSecretsOrExit(ctx context.Context) {
	if err := EnforceSecrets(ctx); err != nil {
		logger.Errorf(ctx, "[startup-env] %v", err)
		fmt.Fprintln(os.Stderr, err.Error())
		exitFunc(1)
	}
}
