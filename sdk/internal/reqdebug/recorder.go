package reqdebug

import (
	"context"
	"sync"
	"time"
)

type callDuration struct {
	OperationType string `json:"operation_type"`
	DurationMS    int64  `json:"duration_ms"`
}

type apiCallDuration struct {
	Source        string `json:"source"`
	OperationType string `json:"operation_type"`
	DurationMS    int64  `json:"duration_ms"`
}

type Recorder struct {
	mu                  sync.Mutex
	operation           string
	startedAt           time.Time
	durationMS          int64
	rpcByType           map[string]int
	apiByType           map[string]int
	apiBySource         map[string]int
	rpcDurationByType   map[string]int64
	apiDurationByType   map[string]int64
	apiDurationBySource map[string]int64
	rpcCalls            []callDuration
	apiCalls            []apiCallDuration
	rpcTotal            int
	apiTotal            int
}

func NewRecorder(operation string) *Recorder {
	return &Recorder{
		operation:           operation,
		startedAt:           time.Now(),
		rpcByType:           map[string]int{},
		apiByType:           map[string]int{},
		apiBySource:         map[string]int{},
		rpcDurationByType:   map[string]int64{},
		apiDurationByType:   map[string]int64{},
		apiDurationBySource: map[string]int64{},
	}
}

func (r *Recorder) RecordRPC(operationType string, duration time.Duration) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rpcTotal++
	r.rpcByType[operationType]++
	ms := duration.Milliseconds()
	r.rpcDurationByType[operationType] += ms
	r.rpcCalls = append(r.rpcCalls, callDuration{OperationType: operationType, DurationMS: ms})
}

func (r *Recorder) RecordAPI(source, operationType string, duration time.Duration) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.apiTotal++
	r.apiBySource[source]++
	r.apiByType[operationType]++
	ms := duration.Milliseconds()
	r.apiDurationBySource[source] += ms
	r.apiDurationByType[operationType] += ms
	r.apiCalls = append(r.apiCalls, apiCallDuration{Source: source, OperationType: operationType, DurationMS: ms})
}

func (r *Recorder) MarkDone() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.durationMS = time.Since(r.startedAt).Milliseconds()
	r.mu.Unlock()
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
	rpcDurationByType := make(map[string]int64, len(r.rpcDurationByType))
	for k, v := range r.rpcDurationByType {
		rpcDurationByType[k] = v
	}
	apiDurationByType := make(map[string]int64, len(r.apiDurationByType))
	for k, v := range r.apiDurationByType {
		apiDurationByType[k] = v
	}
	apiDurationBySource := make(map[string]int64, len(r.apiDurationBySource))
	for k, v := range r.apiDurationBySource {
		apiDurationBySource[k] = v
	}
	durationMS := r.durationMS
	if durationMS == 0 && !r.startedAt.IsZero() {
		durationMS = time.Since(r.startedAt).Milliseconds()
	}
	rpcCalls := make([]callDuration, len(r.rpcCalls))
	copy(rpcCalls, r.rpcCalls)
	apiCalls := make([]apiCallDuration, len(r.apiCalls))
	copy(apiCalls, r.apiCalls)
	return map[string]any{
		"operation":   r.operation,
		"duration_ms": durationMS,
		"rpc": map[string]any{
			"total":               r.rpcTotal,
			"by_operation_type":   rpcByType,
			"duration_by_type_ms": rpcDurationByType,
			"calls":               rpcCalls,
		},
		"api": map[string]any{
			"total":                 r.apiTotal,
			"by_source":             apiBySource,
			"by_operation_type":     apiByType,
			"duration_by_source_ms": apiDurationBySource,
			"duration_by_type_ms":   apiDurationByType,
			"calls":                 apiCalls,
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
