package modelcontext

import (
	"strings"
)

// HandleTable is a typed, invocation-local bidirectional mapping used when a
// prompt needs compact handles for durable values (wiki-ingest ref-N slug
// handles, c000 citation-batch handles). Handles must never be persisted;
// Resolve converts model output back to the durable value first. It is the
// exported wrapper over handleTable.
type HandleTable struct {
	table *handleTable[struct{}]
}

// NewHandleTable creates a handle space such as c000 (prefix=c, width=3,
// start=0) or ref-1 (prefix=ref-, width=0, start=1).
func NewHandleTable(prefix string, width, start int) *HandleTable {
	return &HandleTable{table: newHandleTable[struct{}](prefix, width, start)}
}

// Register returns the stable handle assigned to value in this table.
func (t *HandleTable) Register(value string) string {
	if t == nil {
		return ""
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return t.table.register(value, value, struct{}{}, nil)
}

// Handle returns an already registered handle without creating one.
func (t *HandleTable) Handle(value string) (string, bool) {
	if t == nil {
		return "", false
	}
	return t.table.handleForKey(value)
}

// Resolve converts a known model handle back to its durable value.
func (t *HandleTable) Resolve(handle string) (string, bool) {
	if t == nil {
		return "", false
	}
	value, _, ok := t.table.resolve(strings.TrimSpace(handle))
	return value, ok
}

func (t *HandleTable) Empty() bool {
	return t == nil || t.table.size() == 0
}

func (t *HandleTable) Len() int {
	if t == nil {
		return 0
	}
	return t.table.size()
}
