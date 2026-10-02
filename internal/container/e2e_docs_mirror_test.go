package container

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hibiken/asynq"

	"github.com/magicyuan876/yuheng/internal/docs"
	"github.com/magicyuan876/yuheng/internal/docs/acl"
	"github.com/magicyuan876/yuheng/internal/docs/index"
	"github.com/magicyuan876/yuheng/internal/docs/service"
	"github.com/magicyuan876/yuheng/internal/router"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// processRecorder stands in for the one thing the mirror must not run for real:
// the document-processing worker, which would call the docreader and an
// embedding model. It is registered on the real task executor in place of the
// real handler, so everything up to and including the enqueue is production
// code, and the test sees exactly the task a worker would have received.
type processRecorder struct {
	mu    sync.Mutex
	tasks []types.ManualProcessPayload
}

func (r *processRecorder) handle(_ context.Context, task *asynq.Task) error {
	var p types.ManualProcessPayload
	if err := json.Unmarshal(task.Payload(), &p); err != nil {
		return err
	}
	r.mu.Lock()
	r.tasks = append(r.tasks, p)
	r.mu.Unlock()
	return nil
}

// waitFor returns the tasks once there are n of them. The executor runs a task
// on its own goroutine, so the handler fires shortly after Enqueue returns.
func (r *processRecorder) waitFor(t *testing.T, n int) []types.ManualProcessPayload {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		r.mu.Lock()
		got := append([]types.ManualProcessPayload(nil), r.tasks...)
		r.mu.Unlock()
		if len(got) >= n || time.Now().After(deadline) {
			if len(got) != n {
				t.Fatalf("%d processing task(s) enqueued, want %d: %+v", len(got), n, got)
			}
			return got
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// settle gives a task that should NOT have been enqueued the time it would need
// to show up, then asserts the total is still n.
func (r *processRecorder) settle(t *testing.T, n int) {
	t.Helper()
	time.Sleep(150 * time.Millisecond)
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.tasks) != n {
		t.Fatalf("%d processing task(s) enqueued, want %d: %+v", len(r.tasks), n, r.tasks)
	}
}

// TestDocsMirrorAgainstTheRealKnowledgeService runs the docs module's mirror
// (PageService.SyncPageToKnowledge -> knowledgeBridge -> the real
// KnowledgeService -> real repositories -> PostgreSQL) and asserts on the rows.
//
// The mirror used to be tested against a fake Knowledge, which agrees with
// whatever the author believed. Against the real service two things turned out
// wrong: pages were created as knowledge DRAFTS, which the pipeline never
// chunks or embeds, and they were written with no tenant in the context, so the
// service panicked or wrote to no tenant. Both are pinned here by their effects
// on the database and on the task queue.
//
// Real: docs services, bridge, tenant/KB/knowledge repositories, knowledge
// service, task executor, database. Replaced: the handler of the
// manual-processing task (see processRecorder).
func TestDocsMirrorAgainstTheRealKnowledgeService(t *testing.T) {
	s := bootServer(t, bootOptions{})
	ctx := context.Background()

	rec := &processRecorder{}
	must1(t, s.DI.Invoke(func(exec *router.SyncTaskExecutor) {
		exec.RegisterHandler(types.TypeManualProcess, rec.handle)
	}))

	// A workspace with an owner, and a knowledge base in it.
	user, tenantID := createUserWithWorkspace(t, s, "owner", "owner@example.com", "owner password")
	var mod *docs.Module
	must1(t, s.DI.Invoke(func(m *docs.Module) { mod = m }))
	kb := &types.KnowledgeBase{ID: "kb-docs-mirror", Name: "Handbook KB", TenantID: tenantID, Type: "document"}
	must1(t, s.DB.Create(kb).Error)

	actor, err := mod.Resolver.Identity(ctx, tenantID, user.ID)
	must1(t, err)
	pages := mod.Services.Pages

	space, err := mod.Services.Spaces.Create(ctx, actor, service.CreateSpaceInput{Name: "Handbook"})
	must1(t, err)
	_, err = mod.Services.Spaces.BindKnowledgeBase(ctx, actor, space.Space, &kb.ID, nil)
	must1(t, err)

	newPage := func(title, markdown string) string {
		t.Helper()
		v, err := pages.Create(ctx, actor, service.CreatePageInput{SpaceID: space.ID, Title: title})
		must1(t, err)
		write(t, mod, actor, v.ID, markdown)
		return v.ID
	}
	sync := func(pageID string) *service.IndexResult {
		t.Helper()
		res, err := pages.SyncPageToKnowledge(ctx, tenantID, pageID)
		if err != nil {
			t.Fatalf("SyncPageToKnowledge: %v", err)
		}
		return res
	}
	pageRow := func(pageID string) (knowledgeID string) {
		t.Helper()
		page, err := mod.Repos.Pages.GetAny(ctx, tenantID, pageID)
		must1(t, err)
		if page.KnowledgeID != nil {
			return *page.KnowledgeID
		}
		return ""
	}
	// liveKnowledge counts the knowledge rows a user or the pipeline could see.
	liveKnowledge := func() int64 {
		var n int64
		must1(t, s.DB.Model(&types.Knowledge{}).Where("knowledge_base_id = ?", kb.ID).Count(&n).Error)
		return n
	}

	pageID := newPage("Quota policy", "Every space has a storage quota set by the workspace administrator.")

	t.Run("a page becomes a published, tenant-scoped knowledge row and one processing task", func(t *testing.T) {
		res := sync(pageID)
		if !res.Indexed {
			t.Fatalf("page was not indexed: %+v", res)
		}
		kid := pageRow(pageID)
		if kid == "" {
			t.Fatal("the page does not point at its knowledge entry")
		}

		var row types.Knowledge
		must1(t, s.DB.Where("id = ?", kid).First(&row).Error)
		if row.TenantID != tenantID {
			t.Errorf("tenant_id = %d, want %d: the mirror wrote outside the space's tenant", row.TenantID, tenantID)
		}
		if row.KnowledgeBaseID != kb.ID {
			t.Errorf("knowledge_base_id = %q, want the bound %q", row.KnowledgeBaseID, kb.ID)
		}
		// pending, not draft: the pipeline only picks up "pending" work, and a
		// draft is stored and never becomes searchable.
		if row.ParseStatus != types.ParseStatusPending {
			t.Errorf("parse_status = %q, want %q (draft: never embedded)", row.ParseStatus, types.ParseStatusPending)
		}
		// Search-disabled until the pipeline has embedded it and flips the flag;
		// a half-processed page must not be answerable.
		if row.EnableStatus != "disabled" {
			t.Errorf("enable_status = %q, want disabled until processing completes", row.EnableStatus)
		}
		if row.Channel != docs.DocsChannel || row.Type != types.KnowledgeTypeManual {
			t.Errorf("channel/type = %q/%q, want %q/manual", row.Channel, row.Type, docs.DocsChannel)
		}
		if row.Title != "Quota policy" {
			t.Errorf("title = %q", row.Title)
		}
		meta, err := row.ManualMetadata()
		if err != nil || meta == nil {
			t.Fatalf("manual metadata: %v", err)
		}
		if meta.Status != types.ManualKnowledgeStatusPublish || !strings.Contains(meta.Content, "storage quota") {
			t.Errorf("manual metadata = %+v, want published Markdown of the page", meta)
		}

		tasks := rec.waitFor(t, 1)
		task := tasks[0]
		if task.KnowledgeID != kid || task.TenantID != tenantID || task.KnowledgeBaseID != kb.ID {
			t.Errorf("task addresses %+v, want knowledge %s, tenant %d, kb %s", task, kid, tenantID, kb.ID)
		}
		if task.NeedCleanup {
			t.Error("a new entry has nothing to clean up")
		}
		if !strings.Contains(task.Content, "storage quota") {
			t.Errorf("task carries %q, not the page", task.Content)
		}
	})

	t.Run("syncing an unchanged page sends nothing", func(t *testing.T) {
		before := pageRow(pageID)
		sync(pageID)
		if pageRow(pageID) != before || liveKnowledge() != 1 {
			t.Errorf("an unchanged page changed the mirror (rows: %d)", liveKnowledge())
		}
		rec.settle(t, 1)
	})

	t.Run("an edit updates the same row and enqueues a re-index", func(t *testing.T) {
		before := pageRow(pageID)
		write(t, mod, actor, pageID, "The quota is now set per team, not per space.")
		if res := sync(pageID); !res.Indexed {
			t.Fatalf("edited page not indexed: %+v", res)
		}
		if got := pageRow(pageID); got != before {
			t.Errorf("knowledge id changed from %s to %s: an edit must update, not duplicate", before, got)
		}
		if n := liveKnowledge(); n != 1 {
			t.Errorf("%d knowledge rows for one page", n)
		}
		var row types.Knowledge
		must1(t, s.DB.Where("id = ?", before).First(&row).Error)
		if row.ParseStatus != types.ParseStatusPending || row.TenantID != tenantID {
			t.Errorf("after the update: parse_status=%q tenant=%d", row.ParseStatus, row.TenantID)
		}
		meta, _ := row.ManualMetadata()
		if meta == nil || meta.Version != 2 || !strings.Contains(meta.Content, "per team") {
			t.Errorf("manual metadata after the edit = %+v, want version 2 with the new text", meta)
		}
		tasks := rec.waitFor(t, 2)
		if task := tasks[1]; task.KnowledgeID != before || !task.NeedCleanup {
			t.Errorf("second task = %+v, want the same entry with cleanup of the old chunks", task)
		}
	})

	t.Run("excluding a page removes its row", func(t *testing.T) {
		d, err := mod.Resolver.Page(ctx, actor, pageID)
		must1(t, err)
		_, err = pages.SetKnowledgeExcluded(ctx, actor, d, true)
		must1(t, err)
		res := sync(pageID)
		if res.Indexed || !res.Removed {
			t.Errorf("result = %+v, want removed", res)
		}
		if n := liveKnowledge(); n != 0 {
			t.Errorf("%d knowledge rows left for an excluded page", n)
		}
		if pageRow(pageID) != "" {
			t.Error("the page still points at a removed entry")
		}
	})

	t.Run("restricting an indexed page removes its row", func(t *testing.T) {
		secret := newPage("Salary bands", "Confidential: salary bands by level.")
		if res := sync(secret); !res.Indexed {
			t.Fatalf("not indexed: %+v", res)
		}
		if liveKnowledge() != 1 {
			t.Fatalf("expected the page to be mirrored first, rows=%d", liveKnowledge())
		}
		d, err := mod.Resolver.Page(ctx, actor, secret)
		must1(t, err)
		_, err = pages.SetPageRestricted(ctx, actor, d, true)
		must1(t, err)
		res := sync(secret)
		// Retrieval has no per-entry permission filter, so a restricted page
		// left in the knowledge base would answer anyone allowed to query it.
		if res.Indexed || !res.Removed || res.Reason != index.ReasonRestricted {
			t.Errorf("result = %+v, want removed because restricted", res)
		}
		if n := liveKnowledge(); n != 0 {
			t.Errorf("%d knowledge rows left for a restricted page", n)
		}
	})

	t.Run("a tenant that cannot be loaded is an error and writes nothing", func(t *testing.T) {
		var (
			id        string
			createErr error
		)
		must1(t, s.DI.Invoke(func(svc interfaces.KnowledgeService, tenants interfaces.TenantRepository) {
			b := docs.NewKnowledgeBridge(svc, tenants, nil)
			id, createErr = b.CreateKnowledgeFromText(ctx, 987654321, kb.ID, "Orphan", "Text of a page in no tenant.")
		}))
		err := createErr
		if err == nil {
			t.Fatalf("created knowledge %q for a tenant that does not exist", id)
		}
		if n := liveKnowledge(); n != 0 {
			t.Errorf("%d knowledge rows written for a missing tenant", n)
		}
	})
}

// write replaces a page's content with Markdown, as an editor's save does.
func write(t *testing.T, mod *docs.Module, actor *acl.Identity, pageID, markdown string) {
	t.Helper()
	d, err := mod.Resolver.Page(context.Background(), actor, pageID)
	must1(t, err)
	_, err = mod.Services.Pages.ReplaceContent(context.Background(), actor, d,
		service.ReplaceInput{Markdown: markdown, Reason: "rest"})
	must1(t, err)
}
