// Package extension is the seam through which optional capabilities attach to
// the server without the core knowing what they are.
//
// The core defines what an extension may provide (a feature registry, and
// interfaces added here as the core grows places to plug into), supplies a
// default for each that does nothing, and lets a build add its own. It never
// names, tests for or branches on a particular extension: if a feature is not
// registered, it does not exist, and that is the whole of the contract.
//
// # Adding one
//
// An extension is a package that registers a Hook in its init function and is
// linked into a main package with a blank import:
//
//	func init() {
//		extension.RegisterHook("acme", func(c *dig.Container) error {
//			if err := c.Provide(newAcmeThing); err != nil {
//				return err
//			}
//			// Replace a default the core provided.
//			return c.Decorate(func(base extension.Features, t *acmeThing) extension.Features {
//				return withAcme(base, t)
//			})
//		})
//	}
//
// Hooks run once, in registration order, after the core has registered every
// provider and before anything is resolved (see ApplyHooks), so a hook may both
// add providers and decorate what the core provided.
package extension

import (
	"errors"
	"fmt"
	"maps"
	"sync"
	"sync/atomic"

	"go.uber.org/dig"
)

// Feature names something an extension adds. The names belong to the
// extension: the core neither defines nor interprets them, and only relays
// their status to clients.
type Feature string

// FeatureStatus is what the deployment reports about one feature.
type FeatureStatus struct {
	// Enabled says whether the feature works in this deployment right now.
	Enabled bool
	// Reason says why it does not when Enabled is false: a short,
	// machine-readable code a client can map to a message. Empty when enabled.
	Reason string
}

// Features is the registry of what extensions have added and whether each part
// is usable. Its answers may change while the server runs (a feature can be
// switched on or off without a restart), so callers ask again rather than
// remember.
type Features interface {
	// Status reports one feature. A feature nobody registered is disabled and
	// carries no reason.
	Status(f Feature) FeatureStatus
	// All reports every registered feature. The caller must not modify the
	// result, and must not keep it: it is a snapshot for one response. It is
	// called on each request for the capability list, so it must be cheap and
	// safe for concurrent use.
	All() map[Feature]FeatureStatus
}

// NewFeatures returns the default registry, which has no features. A build
// with no extensions serves this, and an extension replaces it with Decorate.
//
// It refuses to run while extensions are registered but ApplyHooks has not yet
// been called. Whatever is built from a Features at that point would keep the
// default for good (see ApplyHooks), so the mistake is turned into a start-up
// failure instead of a server that quietly ignores its extensions.
func NewFeatures() (Features, error) {
	hooksMu.Lock()
	registered := len(hooks)
	hooksMu.Unlock()
	if registered > 0 && !hooksApplied.Load() {
		return nil, errors.New("extension: Features was built before ApplyHooks ran, " +
			"so it would miss the registered extensions; call ApplyHooks before " +
			"anything that depends on Features is constructed")
	}
	return noFeatures{}, nil
}

type noFeatures struct{}

func (noFeatures) Status(Feature) FeatureStatus { return FeatureStatus{} }

func (noFeatures) All() map[Feature]FeatureStatus { return nil }

// StaticFeatures is a Features that answers from a fixed set. Extensions whose
// availability does not change at runtime can use it directly, and it stands in
// for a real registry in tests.
type StaticFeatures map[Feature]FeatureStatus

// Status implements Features.
func (s StaticFeatures) Status(f Feature) FeatureStatus { return s[f] }

// All implements Features. It returns a copy, so a caller that modifies the
// result cannot change what the registry reports.
func (s StaticFeatures) All() map[Feature]FeatureStatus { return maps.Clone(s) }

// Hook registers an extension's providers and decorators with the container.
type Hook func(c *dig.Container) error

type namedHook struct {
	name string
	run  Hook
}

var (
	hooksMu sync.Mutex
	hooks   []namedHook

	// hooksApplied records that ApplyHooks has run to completion, for the check
	// in NewFeatures.
	hooksApplied atomic.Bool
)

// RegisterHook adds a hook to run when the container is built. It is meant to
// be called from an init function, where a mistake should stop the program at
// start-up rather than surface later, so a missing name, a nil hook or a name
// registered twice panics.
func RegisterHook(name string, h Hook) {
	if name == "" {
		panic("extension: RegisterHook called with an empty name")
	}
	if h == nil {
		panic(fmt.Sprintf("extension: RegisterHook %q called with a nil hook", name))
	}
	hooksMu.Lock()
	defer hooksMu.Unlock()
	for _, existing := range hooks {
		if existing.name == name {
			panic(fmt.Sprintf("extension: hook %q registered twice", name))
		}
	}
	hooks = append(hooks, namedHook{name: name, run: h})
}

// ApplyHooks runs the registered hooks against the container, in the order they
// were registered. The first hook to fail stops the rest and its error names
// it, so a broken extension is identified at start-up.
//
// Call it after the core has provided everything and before anything that
// depends on a decorated type is constructed. dig does not refuse a decoration
// that comes late; it quietly leaves alone every value that was already built
// with the undecorated one. A router constructed first would go on serving the
// default for the life of the process, with no error to say so.
func ApplyHooks(c *dig.Container) error {
	hooksMu.Lock()
	pending := make([]namedHook, len(hooks))
	copy(pending, hooks)
	hooksMu.Unlock()

	for _, h := range pending {
		if err := h.run(c); err != nil {
			return fmt.Errorf("extension %q: %w", h.name, err)
		}
	}
	hooksApplied.Store(true)
	return nil
}
