package httpx

import "context"

type ctxKey int

const (
	ctxCorrelationID ctxKey = iota
	ctxPrincipal
)

func CorrelationID(ctx context.Context) string {
	if v, ok := ctx.Value(ctxCorrelationID).(string); ok {
		return v
	}
	return ""
}

// Principal is the authenticated identity extracted from a validated token.
type Principal struct {
	Subject string
	Email   string
	Groups  []string
}

func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxPrincipal).(Principal)
	return p, ok
}
