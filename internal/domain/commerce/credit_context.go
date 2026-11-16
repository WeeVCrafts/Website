package commerce

import "context"

type creditPreferenceKey struct{}
type creditQuoteKey struct{}

func WithExpectedCredit(ctx context.Context, amount int64) context.Context {
	return context.WithValue(ctx, creditQuoteKey{}, amount)
}
func ExpectedCredit(ctx context.Context) (int64, bool) {
	amount, ok := ctx.Value(creditQuoteKey{}).(int64)
	return amount, ok
}

func WithStoreCredit(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, creditPreferenceKey{}, enabled)
}
func UsesStoreCredit(ctx context.Context) bool {
	enabled, _ := ctx.Value(creditPreferenceKey{}).(bool)
	return enabled
}
