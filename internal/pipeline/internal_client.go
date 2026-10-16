package pipeline

import "context"

// SkippableStage is the only stage an internal client may skip (#87).
const SkippableStage = "redact"

// InternalClient is an identity tollgate sets for its own requests (the
// speedtest, #87). It exists only inside the process: the context key is
// unexported, so no HTTP header or body can produce it.
type InternalClient struct {
	ID string
	// SkipRedact skips the redact stage; no other stage can be skipped.
	SkipRedact bool
}

type internalClientKey struct{}

// WithInternalClient returns ctx carrying the internal identity c.
func WithInternalClient(ctx context.Context, c InternalClient) context.Context {
	return context.WithValue(ctx, internalClientKey{}, c)
}

func internalClient(ctx context.Context) (InternalClient, bool) {
	c, ok := ctx.Value(internalClientKey{}).(InternalClient)
	return c, ok && c.ID != ""
}
