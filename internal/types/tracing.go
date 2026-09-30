package types

// TracingContext is an embeddable struct that carries observability context
// (currently Langfuse trace/span ids plus user/session hints) across process
// boundaries — specifically, from an HTTP request into an asynq task payload
// and back out inside the worker.
//
// It lives in the types package, not the langfuse package, so that:
//
//   - asynq payload structs can embed it without pulling the langfuse
//     package into every service/handler import graph;
//   - the langfuse package can remain a leaf dependency that only types
//     (and its own tests) reference directly.
//
// The JSON tags all use the "lf_" prefix and omitempty so that untraced
// payloads carry nothing extra (empty fields collapse to nothing in the
// serialized output) and so that Langfuse-specific keys don't collide with
// business fields that may happen to be named similarly.
type TracingContext struct {
	// LangfuseTraceID is the id of the root trace that originated this task.
	// Span linking itself flows through LangfuseTraceparent below; this plain
	// id is what the document-processing tracker records on each attempt so
	// the attempt can be opened in Langfuse directly.
	LangfuseTraceID string `json:"lf_trace_id,omitempty"`
	// LangfuseTraceparent carries the W3C Trace Context (`traceparent` header
	// value: `00-<trace_id>-<span_id>-<flags>`) from the enqueuing request.
	// The worker re-extracts it so its spans are children of the same trace —
	// stitching the HTTP request and the async job into one LiteFuse tree. This
	// is also what propagates a sop3 run's W3C trace_id into any asynq jobs
	// Yuheng enqueues while serving sop3's agent-chat call.
	LangfuseTraceparent string `json:"lf_traceparent,omitempty"`
	// LangfuseUserID preserves the userId / tenant label across the async
	// boundary so that orphan async traces (when no upstream trace id is
	// available) still show up in the Langfuse "Users" view under the
	// right tenant.
	LangfuseUserID string `json:"lf_user_id,omitempty"`
	// LangfuseSessionID preserves the sessionId for the same reason.
	LangfuseSessionID string `json:"lf_session_id,omitempty"`
}

// SetLangfuseTracing overwrites the embedded TracingContext. Method is
// exported so helpers in internal/tracing/langfuse can populate it via the
// LangfuseTracingCarrier interface without reflection.
func (tc *TracingContext) SetLangfuseTracing(other TracingContext) {
	*tc = other
}

// GetLangfuseTracing returns a copy of the embedded TracingContext.
func (tc TracingContext) GetLangfuseTracing() TracingContext {
	return tc
}

// LangfuseTracingCarrier is implemented automatically by any struct that
// embeds TracingContext (thanks to Go's method promotion rules). The asynq
// enqueue helper in internal/tracing/langfuse uses this interface to inject
// the current trace/span ids into the payload without caring about the
// concrete payload type.
type LangfuseTracingCarrier interface {
	SetLangfuseTracing(TracingContext)
	GetLangfuseTracing() TracingContext
}
