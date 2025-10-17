package omcrypto

import "context"

type (
	contextKey int
)

const (
	cryptoKey contextKey = 0
)

func ContextWithCrypto(ctx context.Context, c <-chan *Factory) context.Context {
	return context.WithValue(ctx, cryptoKey, c)
}

func CryptoFromContext(ctx context.Context) <-chan *Factory {
	if c, ok := ctx.Value(cryptoKey).(<-chan *Factory); ok {
		return c
	}
	panic("context has no crypto")
}
