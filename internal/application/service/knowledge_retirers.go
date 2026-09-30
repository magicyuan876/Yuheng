package service

import (
	"sync"

	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// knowledgeRetirers is the registry of retirers by origin. It exists so that a
// module the findings service must not depend on — the docs module, which
// depends on the findings service itself — can still say how its entries are
// retired: the module registers into the registry when it is built, and the
// service looks up when a person supersedes something.
type knowledgeRetirers struct {
	mu       sync.RWMutex
	byOrigin map[types.KnowledgeOrigin]interfaces.KnowledgeRetirer
}

// NewKnowledgeRetirers returns an empty registry.
func NewKnowledgeRetirers() interfaces.KnowledgeRetirers {
	return &knowledgeRetirers{byOrigin: map[types.KnowledgeOrigin]interfaces.KnowledgeRetirer{}}
}

// Register implements interfaces.KnowledgeRetirers. A second registration for
// an origin replaces the first.
func (r *knowledgeRetirers) Register(origin types.KnowledgeOrigin, retirer interfaces.KnowledgeRetirer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byOrigin[origin] = retirer
}

// For implements interfaces.KnowledgeRetirers.
func (r *knowledgeRetirers) For(origin types.KnowledgeOrigin) (interfaces.KnowledgeRetirer, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	retirer, ok := r.byOrigin[origin]
	return retirer, ok && retirer != nil
}
