package ports

import "context"

type orderWorkspaceMetadataKey struct{}

// WithOrderWorkspaceMetadata requests bounded customer-safe detail collections.
// It is set by the owned-order API, never by an untrusted request header.
func WithOrderWorkspaceMetadata(ctx context.Context) context.Context {
	return context.WithValue(ctx, orderWorkspaceMetadataKey{}, true)
}
func OrderWorkspaceMetadataRequested(ctx context.Context) bool {
	v, _ := ctx.Value(orderWorkspaceMetadataKey{}).(bool)
	return v
}
