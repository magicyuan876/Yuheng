package audit_test

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// Every action name in the vocabulary is either recorded somewhere or listed
// below as owed.
//
// T4.4's contract audit found four that were not: the page row had carried
// is_locked and status since T0.2 and the resolver had capped a locked page
// at reader since T0.3, but nothing could set either field — a permission
// rule no code path could trigger, which reads as working. A declared action
// that nothing records is the cheapest available signal of that shape of
// gap, so it is worth a test rather than a periodic grep.

// notYetRecorded are actions whose feature has not been built, with the work
// package that will record them. Deleting an entry is part of doing the work.
var notYetRecorded = map[string]string{
	"Imported": "T6 (import)",
}

var (
	declaration = regexp.MustCompile(`(?m)^\t(\w+)\s+types\.AuditAction\s*=`)
	usage       = regexp.MustCompile(`audit\.(\w+)`)
)

// moduleRoot is internal/docs, the tree that may record these actions.
func moduleRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate this test file")
	}
	return filepath.Dir(filepath.Dir(thisFile))
}

func TestEveryAuditActionIsRecordedSomewhere(t *testing.T) {
	root := moduleRoot(t)

	declared := map[string]bool{}
	raw, err := os.ReadFile(filepath.Join(root, "audit", "actions.go"))
	if err != nil {
		t.Fatalf("read actions.go: %v", err)
	}
	for _, m := range declaration.FindAllStringSubmatch(string(raw), -1) {
		declared[m[1]] = true
	}
	if len(declared) < 20 {
		t.Fatalf("found only %d action declarations; the parser has drifted from actions.go",
			len(declared))
	}

	used := map[string]bool{}
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		// The declarations themselves are not uses, and neither is this test.
		if strings.HasSuffix(path, "actions.go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range usage.FindAllStringSubmatch(string(body), -1) {
			used[m[1]] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}

	var unrecorded []string
	for name := range declared {
		if used[name] {
			continue
		}
		if _, owed := notYetRecorded[name]; owed {
			continue
		}
		unrecorded = append(unrecorded, name)
	}
	if len(unrecorded) > 0 {
		sort.Strings(unrecorded)
		t.Errorf("%d audit actions are declared but never recorded:\n  %s\n\n"+
			"Either the feature is missing (record it), the action is dead (delete it), "+
			"or the work is still owed (add it to notYetRecorded with its work package).",
			len(unrecorded), strings.Join(unrecorded, "\n  "))
	}
}

// The other direction: an owed action that started being recorded leaves a
// stale promise in the list above.
func TestOwedAuditActionsAreStillOwed(t *testing.T) {
	root := moduleRoot(t)

	used := map[string][]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		if strings.HasSuffix(path, "actions.go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range usage.FindAllStringSubmatch(string(body), -1) {
			used[m[1]] = append(used[m[1]], filepath.Base(path))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}

	for name, owner := range notYetRecorded {
		if where, recorded := used[name]; recorded {
			t.Errorf("%s is recorded now (in %s) — remove it from notYetRecorded (%s)",
				name, strings.Join(where, ", "), owner)
		}
	}
}
