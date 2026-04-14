package market

import (
	"context"

	"github.com/TokensHive/solana-token-market-go/sdk/internal/reqdebug"
)

type Service struct {
	cfg         Config
	calculators map[PoolRoute]PoolCalculator
}

func NewService(cfg Config) *Service {
	s := &Service{
		cfg:         cfg,
		calculators: map[PoolRoute]PoolCalculator{},
	}
	for route, factory := range defaultPoolCalculatorFactories() {
		if factory != nil {
			s.calculators[route] = factory(cfg)
		}
	}
	for route, factory := range cfg.PoolCalculatorFactories {
		if factory != nil {
			s.calculators[route] = factory(cfg)
		}
	}
	return s
}

func attachRequestDebug(ctx context.Context, meta map[string]any) map[string]any {
	recorder := reqdebug.FromContext(ctx)
	if recorder == nil {
		return meta
	}
	if meta == nil {
		meta = map[string]any{}
	}
	meta["requests"] = recorder.SnapshotMap()
	return meta
}
