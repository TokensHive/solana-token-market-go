package reqdebug

import (
	"context"
	"sync"
)

type Recorder struct {
	mu          sync.Mutex
	operation   string
	rpcByType   map[string]int
	apiByType   map[string]int
	apiBySource map[string]int
	rpcTotal    int
	apiTotal    int
}

func NewRecorder(operation string) *Recorder {
	return &Recorder{
		operation:   operation,
		rpcByType:   map[string]int{},
		apiByType:   map[string]int{},
		apiBySource: map[string]int{},
	}
}

func (r *Recorder) RecordRPC(operationType string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rpcTotal++
	r.rpcByType[operationType]++
}

func (r *Recorder) RecordAPI(source, operationType string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.apiTotal++
	r.apiBySource[source]++
	r.apiByType[operationType]++
}

func (r *Recorder) SnapshotMap() map[string]any {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	rpcByType := make(map[string]int, len(r.rpcByType))
	for k, v := range r.rpcByType {
		rpcByType[k] = v
	}
	apiByType := make(map[string]int, len(r.apiByType))
	for k, v := range r.apiByType {
		apiByType[k] = v
	}
	apiBySource := make(map[string]int, len(r.apiBySource))
	for k, v := range r.apiBySource {
		apiBySource[k] = v
	}
	return map[string]any{
		"operation": r.operation,
		"rpc": map[string]any{
			"total":             r.rpcTotal,
			"by_operation_type": rpcByType,
		},
		"api": map[string]any{
			"total":             r.apiTotal,
			"by_source":         apiBySource,
			"by_operation_type": apiByType,
		},
	}
}

type ctxKey struct{}

func WithRecorder(ctx context.Context, recorder *Recorder) context.Context {
	if recorder == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, recorder)
}

func FromContext(ctx context.Context) *Recorder {
	v := ctx.Value(ctxKey{})
	recorder, _ := v.(*Recorder)
	return recorder
}
