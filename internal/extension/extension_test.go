package extension

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"
)

func resetHooks(t *testing.T) { IsolateForTest(t) }

func TestDefaultFeaturesHaveNothing(t *testing.T) {
	resetHooks(t)
	f, err := NewFeatures()
	require.NoError(t, err)

	assert.Empty(t, f.All())
	assert.Equal(t, FeatureStatus{}, f.Status("anything"),
		"a feature nobody registered is disabled and has no reason")
}

func TestStaticFeaturesReportWhatTheyHold(t *testing.T) {
	f := StaticFeatures{
		"on":  {Enabled: true},
		"off": {Reason: "needs_something"},
	}

	assert.True(t, f.Status("on").Enabled)
	assert.Equal(t, FeatureStatus{Reason: "needs_something"}, f.Status("off"))
	assert.Equal(t, FeatureStatus{}, f.Status("unknown"))
	assert.Len(t, f.All(), 2)
}

func TestStaticFeaturesAllReturnsACopy(t *testing.T) {
	f := StaticFeatures{"a": {Enabled: true}}

	all := f.All()
	all["a"] = FeatureStatus{}
	all["b"] = FeatureStatus{Enabled: true}

	assert.True(t, f.Status("a").Enabled, "changing the result must not change the registry")
	assert.False(t, f.Status("b").Enabled)
}

func TestHooksRunInRegistrationOrder(t *testing.T) {
	resetHooks(t)
	var order []string
	for _, name := range []string{"first", "second", "third"} {
		RegisterHook(name, func(*dig.Container) error {
			order = append(order, name)
			return nil
		})
	}

	require.NoError(t, ApplyHooks(dig.New()))

	assert.Equal(t, []string{"first", "second", "third"}, order)
}

func TestAFailingHookStopsTheRestAndIsNamed(t *testing.T) {
	resetHooks(t)
	boom := errors.New("boom")
	ran := false
	RegisterHook("ok", func(*dig.Container) error { return nil })
	RegisterHook("broken", func(*dig.Container) error { return boom })
	RegisterHook("after", func(*dig.Container) error { ran = true; return nil })

	err := ApplyHooks(dig.New())

	require.Error(t, err)
	assert.ErrorIs(t, err, boom)
	assert.Contains(t, err.Error(), `"broken"`, "the failing extension is identified")
	assert.False(t, ran, "hooks after the failure do not run")
}

func TestNoHooksIsNotAnError(t *testing.T) {
	resetHooks(t)

	assert.NoError(t, ApplyHooks(dig.New()))
}

func TestRegisterHookRejectsMistakes(t *testing.T) {
	resetHooks(t)
	RegisterHook("once", func(*dig.Container) error { return nil })

	assert.PanicsWithValue(t, "extension: RegisterHook called with an empty name",
		func() { RegisterHook("", func(*dig.Container) error { return nil }) })
	assert.PanicsWithValue(t, `extension: RegisterHook "nil" called with a nil hook`,
		func() { RegisterHook("nil", nil) })
	assert.PanicsWithValue(t, `extension: hook "once" registered twice`,
		func() { RegisterHook("once", func(*dig.Container) error { return nil }) })
}

// The point of the package: an extension replaces what the core provided, and
// everything downstream resolves the replacement.
func TestAHookCanReplaceTheDefaultFeatures(t *testing.T) {
	resetHooks(t)
	c := dig.New()
	require.NoError(t, c.Provide(NewFeatures))
	RegisterHook("acme", func(c *dig.Container) error {
		return c.Decorate(func(Features) Features {
			return StaticFeatures{"acme.widget": {Enabled: true}}
		})
	})

	require.NoError(t, ApplyHooks(c))

	require.NoError(t, c.Invoke(func(f Features) {
		assert.True(t, f.Status("acme.widget").Enabled)
	}))
}

// A hook may add providers of its own and use them in the decoration.
func TestAHookCanProvideDependenciesForItsDecoration(t *testing.T) {
	resetHooks(t)
	type licence struct{ on bool }
	c := dig.New()
	require.NoError(t, c.Provide(NewFeatures))
	RegisterHook("acme", func(c *dig.Container) error {
		if err := c.Provide(func() *licence { return &licence{on: true} }); err != nil {
			return err
		}
		return c.Decorate(func(_ Features, l *licence) Features {
			return StaticFeatures{"acme.widget": {Enabled: l.on}}
		})
	})

	require.NoError(t, ApplyHooks(c))

	require.NoError(t, c.Invoke(func(f Features) {
		assert.True(t, f.Status("acme.widget").Enabled)
	}))
}

// Pins the failure mode ApplyHooks warns about: a hook that runs after a
// consumer has been built does not reach it, and dig raises no error. The next
// test shows how the package turns that into a start-up failure.
func TestDecorationDoesNotReachConsumersBuiltEarlier(t *testing.T) {
	resetHooks(t)
	type consumer struct{ features Features }
	c := dig.New()
	require.NoError(t, c.Provide(NewFeatures))
	require.NoError(t, c.Provide(func(f Features) *consumer { return &consumer{features: f} }))
	var early *consumer
	require.NoError(t, c.Invoke(func(x *consumer) { early = x }))
	RegisterHook("late", func(c *dig.Container) error {
		return c.Decorate(func(Features) Features {
			return StaticFeatures{"acme.widget": {Enabled: true}}
		})
	})

	require.NoError(t, ApplyHooks(c), "dig accepts the decoration; it just does not reach the earlier consumer")

	assert.False(t, early.features.Status("acme.widget").Enabled)
	require.NoError(t, c.Invoke(func(f Features) {
		assert.True(t, f.Status("acme.widget").Enabled, "a fresh request for the type does see it")
	}))
}

// With extensions registered, building Features before ApplyHooks is a bug that
// would otherwise show up as an extension that never takes effect. It fails
// loudly instead.
func TestFeaturesRefuseToBeBuiltBeforeTheHooksRan(t *testing.T) {
	resetHooks(t)
	RegisterHook("acme", func(c *dig.Container) error {
		return c.Decorate(func(Features) Features {
			return StaticFeatures{"acme.widget": {Enabled: true}}
		})
	})
	c := dig.New()
	require.NoError(t, c.Provide(NewFeatures))

	err := c.Invoke(func(Features) {})
	require.Error(t, err, "resolving Features first is the ordering mistake")
	assert.Contains(t, err.Error(), "ApplyHooks")

	require.NoError(t, ApplyHooks(c))
	require.NoError(t, c.Invoke(func(f Features) {
		assert.True(t, f.Status("acme.widget").Enabled)
	}))
}

// A build with no extensions has nothing to apply, so the order does not matter.
func TestFeaturesAreFreelyBuildableWhenNothingIsRegistered(t *testing.T) {
	resetHooks(t)
	c := dig.New()
	require.NoError(t, c.Provide(NewFeatures))

	require.NoError(t, c.Invoke(func(f Features) { assert.Empty(t, f.All()) }))
}
