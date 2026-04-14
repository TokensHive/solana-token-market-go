package reqdebug

import (
	"context"
	"testing"
	"time"
)

func TestRecorderSnapshot(t *testing.T) {
	r := NewRecorder("op")
	r.RecordRPC("get_account", 12*time.Millisecond)
	r.RecordAPI("sourceA", "fetch", 20*time.Millisecond)
	r.MarkDone()

	snapshot := r.SnapshotMap()
	if snapshot["operation"] != "op" {
		t.Fatalf("unexpected operation: %v", snapshot["operation"])
	}
	if snapshot["duration_ms"].(int64) < 0 {
		t.Fatalf("unexpected duration: %v", snapshot["duration_ms"])
	}

	rpcBlock := snapshot["rpc"].(map[string]any)
	if rpcBlock["total"].(int) != 1 {
		t.Fatalf("expected one rpc call, got %v", rpcBlock["total"])
	}
	apiBlock := snapshot["api"].(map[string]any)
	if apiBlock["total"].(int) != 1 {
		t.Fatalf("expected one api call, got %v", apiBlock["total"])
	}
}

func TestRecorderNilSafeMethods(t *testing.T) {
	var r *Recorder
	r.RecordRPC("x", time.Millisecond)
	r.RecordAPI("s", "x", time.Millisecond)
	r.MarkDone()
	if snap := r.SnapshotMap(); snap != nil {
		t.Fatalf("expected nil snapshot, got %#v", snap)
	}
}

func TestRecorderContextHelpers(t *testing.T) {
	ctx := context.Background()
	if got := FromContext(ctx); got != nil {
		t.Fatalf("expected no recorder in plain context, got %#v", got)
	}
	ctx = WithRecorder(ctx, nil)
	if got := FromContext(ctx); got != nil {
		t.Fatalf("expected nil recorder when attaching nil, got %#v", got)
	}

	rec := NewRecorder("ctx")
	ctx = WithRecorder(ctx, rec)
	if got := FromContext(ctx); got != rec {
		t.Fatal("expected same recorder from context")
	}
}

func TestRecorderSnapshotWithoutMarkDone(t *testing.T) {
	r := NewRecorder("timed")
	time.Sleep(1 * time.Millisecond)
	snapshot := r.SnapshotMap()
	if snapshot["duration_ms"].(int64) <= 0 {
		t.Fatalf("expected positive computed duration, got %v", snapshot["duration_ms"])
	}
}
