package container

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.uber.org/dig"

	"github.com/magicyuan876/yuheng/internal/application/service/retriever"
	"github.com/magicyuan876/yuheng/internal/extension"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	"github.com/magicyuan876/yuheng/internal/utils"
)

const stubNetworkEngine types.RetrieverEngineType = "stubnet"

// stubNetworkDescriptor is a registrable engine that "dials" the address in a
// store's Addr. built reports whether its constructor ran.
func stubNetworkDescriptor(built *bool) retriever.EngineDescriptor {
	return retriever.EngineDescriptor{
		Type:        stubNetworkEngine,
		Driver:      "stubnet",
		DisplayName: "Stub network engine",
		Capabilities: retriever.EngineCapabilities{
			Retrievers: []types.RetrieverType{types.VectorRetrieverType},
		},
		Registrable:   true,
		DialAddresses: func(cc types.ConnectionConfig) []string { return []string{cc.Addr} },
		TestConnection: func(context.Context, types.ConnectionConfig) (string, error) {
			return "", nil
		},
		EnvStore: func(types.EnvLookupFunc) *types.VectorStore { return nil },
		New: func(context.Context, types.VectorStore, retriever.EngineDeps) (interfaces.RetrieveEngineService, error) {
			*built = true
			return nil, errors.New("stub engine is not a real engine")
		},
	}
}

func newTestFactory(t *testing.T, built *bool) interfaces.EngineFactory {
	t.Helper()
	catalog, err := retriever.NewCatalog(retriever.PostgresDescriptor(), stubNetworkDescriptor(built))
	if err != nil {
		t.Fatalf("NewCatalog: %v", err)
	}
	return NewEngineFactory(nil, nil, nil, catalog)
}

func TestNewEngineFactory_FailsClosed(t *testing.T) {
	t.Run("a non-registrable engine is refused", func(t *testing.T) {
		var built bool
		factory := newTestFactory(t, &built)
		_, err := factory(context.Background(), types.VectorStore{
			EngineType:       types.PostgresRetrieverEngineType,
			ConnectionConfig: types.ConnectionConfig{UseDefaultConnection: true},
		})
		if err == nil || !strings.Contains(err.Error(), "cannot be registered") {
			t.Fatalf("expected a not-registrable error, got %v", err)
		}
	})

	t.Run("an unknown engine is refused", func(t *testing.T) {
		var built bool
		factory := newTestFactory(t, &built)
		_, err := factory(context.Background(), types.VectorStore{EngineType: "elasticsearch"})
		if err == nil || !strings.Contains(err.Error(), "not available") {
			t.Fatalf("expected a not-available error, got %v", err)
		}
		if built {
			t.Fatal("no constructor may run for an unknown engine")
		}
	})

	t.Run("private and loopback addresses are refused before the engine is built", func(t *testing.T) {
		// TestMain whitelists loopback for the wiring tests; SSRF policy is the
		// subject here, so switch the whitelist off for this test only.
		utils.SetSSRFWhitelistFromRaw("")
		t.Cleanup(func() { utils.SetSSRFWhitelistFromRaw("127.0.0.1,::1,localhost") })

		for _, addr := range []string{
			"http://127.0.0.1:9200",
			"http://10.0.0.5:9200",
			"192.168.1.10:9030",
			"169.254.169.254:80",
		} {
			var built bool
			factory := newTestFactory(t, &built)
			_, err := factory(context.Background(), types.VectorStore{
				EngineType:       stubNetworkEngine,
				ConnectionConfig: types.ConnectionConfig{Addr: addr},
			})
			if err == nil || !strings.Contains(err.Error(), "SSRF") {
				t.Fatalf("%s: expected an SSRF error, got %v", addr, err)
			}
			if built {
				t.Fatalf("%s: the engine was constructed despite the SSRF failure", addr)
			}
		}
	})

	t.Run("a whitelisted address reaches the constructor", func(t *testing.T) {
		// The counterpart of the test above: it proves the refusals come from
		// the address policy and not from the stub being unusable.
		var built bool
		factory := newTestFactory(t, &built)
		_, err := factory(context.Background(), types.VectorStore{
			EngineType:       stubNetworkEngine,
			ConnectionConfig: types.ConnectionConfig{Addr: "http://127.0.0.1:9200"},
		})
		if !built {
			t.Fatalf("expected the constructor to run, got %v", err)
		}
	})
}

func TestNewEngineCatalog_RequiresExtensionHooksToBeApplied(t *testing.T) {
	extension.IsolateForTest(t)
	extension.RegisterHook("container-test-noop", func(*dig.Container) error { return nil })

	params := engineCatalogParams{Descriptors: []retriever.EngineDescriptor{retriever.PostgresDescriptor()}}

	catalog, err := NewEngineCatalog(params)
	if err == nil || !strings.Contains(err.Error(), "ApplyHooks") {
		t.Fatalf("expected a build-order error before ApplyHooks ran, got catalog=%v err=%v", catalog, err)
	}
	if catalog != nil {
		t.Fatal("no catalog may be returned when the build order is wrong")
	}

	if err := extension.ApplyHooks(dig.New()); err != nil {
		t.Fatalf("ApplyHooks: %v", err)
	}
	catalog, err = NewEngineCatalog(params)
	if err != nil {
		t.Fatalf("NewEngineCatalog after ApplyHooks: %v", err)
	}
	if _, ok := catalog.ByType(types.PostgresRetrieverEngineType); !ok {
		t.Fatal("the catalog lacks the descriptor it was given")
	}
}
