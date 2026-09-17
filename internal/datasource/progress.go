package datasource

import "context"

// ProgressFunc receives coarse in-flight sync progress: a machine-readable
// action key (localized by the frontend, e.g. "download_video",
// "process_item") and a human-readable target such as a file or document
// title. Implementations must be cheap; reporters may be called from the hot
// per-item path.
type ProgressFunc func(action, target string)

type progressCtxKey struct{}

// WithProgress returns a context carrying a progress reporter for a sync run.
func WithProgress(ctx context.Context, f ProgressFunc) context.Context {
	return context.WithValue(ctx, progressCtxKey{}, f)
}

// ReportProgress invokes the context's progress reporter, if any. It is a safe
// no-op otherwise, so connectors can report unconditionally.
func ReportProgress(ctx context.Context, action, target string) {
	if f, ok := ctx.Value(progressCtxKey{}).(ProgressFunc); ok && f != nil {
		f(action, target)
	}
}
