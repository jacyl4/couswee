package usage

import "context"

type RefreshReason string

const (
	RefreshReasonManual        RefreshReason = "manual"
	RefreshReasonStartup       RefreshReason = "startup"
	RefreshReasonPeriodic      RefreshReason = "periodic"
	RefreshReasonAccountAdded  RefreshReason = "account-added"
	RefreshReasonAccountSwitch RefreshReason = "account-switch"
	RefreshReasonLoginSuccess  RefreshReason = "login-success"
)

func (r RefreshReason) String() string {
	if r == "" {
		return string(RefreshReasonManual)
	}
	return string(r)
}

type refreshReasonContextKey struct{}

func ContextWithRefreshReason(ctx context.Context, reason RefreshReason) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, refreshReasonContextKey{}, reasonOrManual(reason))
}

func RefreshReasonFromContext(ctx context.Context) RefreshReason {
	if ctx == nil {
		return RefreshReasonManual
	}
	reason, _ := ctx.Value(refreshReasonContextKey{}).(RefreshReason)
	return reasonOrManual(reason)
}

func reasonOrManual(reason RefreshReason) RefreshReason {
	if reason == "" {
		return RefreshReasonManual
	}
	return reason
}
