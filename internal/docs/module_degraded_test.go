package docs

import (
	"reflect"
	"sort"
	"testing"
)

// The container's start-up test asserts Module.Degraded is empty. That only
// means something while degraded() looks at every optional field of Params, so
// pin the two together: a new `optional:"true"` field that degraded() forgets
// would never show up in Module.Degraded and its absence would go unnoticed.
func TestDegradedNamesEveryOptionalParam(t *testing.T) {
	var optional []string
	typ := reflect.TypeOf(Params{})
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		// Redis is optional by design: no Redis is a supported deployment.
		if f.Tag.Get("optional") == "true" && f.Name != "Redis" {
			optional = append(optional, f.Name)
		}
	}

	// A zero Params has every dependency missing.
	got := Params{}.degraded()
	sort.Strings(got)
	sort.Strings(optional)
	if !reflect.DeepEqual(got, optional) {
		t.Errorf("degraded() reports %v for an empty Params, but the optional fields are %v", got, optional)
	}
}
