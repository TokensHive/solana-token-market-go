package parser

import "errors"

var ErrNoEvidence = errors.New("no parser evidence")

type Edge struct {
	PoolAddress string
	BaseMint    string
	QuoteMint   string
	Protocol    string
	Event       string
	Slot        uint64
}

type Adapter interface {
	FindPoolsByMint(mint string) ([]Edge, error)
	FindPoolByAddress(address string) (*Edge, error)
}

type noopAdapter struct{}

func NewNoopAdapter() Adapter                                  { return &noopAdapter{} }
func (n *noopAdapter) FindPoolsByMint(string) ([]Edge, error)  { return nil, ErrNoEvidence }
func (n *noopAdapter) FindPoolByAddress(string) (*Edge, error) { return nil, ErrNoEvidence }
