package market

import "context"

type PoolRoute struct {
	Dex         Dex
	PoolVersion PoolVersion
}

type PoolCalculator interface {
	Compute(ctx context.Context, pool PoolIdentifier) (*GetMetricsByPoolResponse, error)
}

type PoolCalculatorFactory func(cfg Config) PoolCalculator

type poolCalculatorFunc func(ctx context.Context, pool PoolIdentifier) (*GetMetricsByPoolResponse, error)

func (f poolCalculatorFunc) Compute(ctx context.Context, pool PoolIdentifier) (*GetMetricsByPoolResponse, error) {
	return f(ctx, pool)
}
