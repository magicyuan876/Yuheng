package datasource

import (
	"strings"
	"testing"

	"github.com/magicyuan876/yuheng/internal/types"
)

func TestFeishuMetadataDoesNotAdvertiseWebhook(t *testing.T) {
	meta := ConnectorMetadataRegistry[types.ConnectorTypeFeishu]

	for _, capability := range meta.Capabilities {
		if capability == "webhook" {
			t.Fatalf("Feishu connector should not advertise webhook until webhook sync is implemented")
		}
	}
}

// metaConnector is a connector that does nothing but carry a type.
type metaConnector struct {
	Connector
	typ string
}

func (c metaConnector) Type() string { return c.typ }

// The types endpoint lists what is registered, in priority order, and never
// a type that has no connector.
func TestAvailableListsOnlyRegisteredConnectors(t *testing.T) {
	r := NewConnectorRegistry()
	for _, typ := range []string{types.ConnectorTypeRSS, types.ConnectorTypeFeishu, types.ConnectorTypeNotion} {
		if err := r.Register(metaConnector{typ: typ}); err != nil {
			t.Fatal(err)
		}
	}
	var got []string
	for _, meta := range r.Available() {
		got = append(got, meta.Type)
	}
	want := []string{types.ConnectorTypeFeishu, types.ConnectorTypeNotion, types.ConnectorTypeRSS}
	if len(got) != len(want) {
		t.Fatalf("Available() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Available() = %v, want %v", got, want)
		}
	}
}

// A registry whose connectors and metadata differ is reported both ways.
func TestVerifyMetadataReportsBothKindsOfDrift(t *testing.T) {
	r := NewConnectorRegistry()
	if err := r.Register(metaConnector{typ: "no-metadata"}); err != nil {
		t.Fatal(err)
	}
	err := r.VerifyMetadata()
	if err == nil {
		t.Fatal("want an error")
	}
	for _, part := range []string{`"no-metadata" has no entry`, `lists "feishu", which has no connector`} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("error %q does not mention %s", err, part)
		}
	}
}
