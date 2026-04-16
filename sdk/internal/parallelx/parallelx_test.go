package parallelx

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRun_NoTasks(t *testing.T) {
	if err := Run(context.Background()); err != nil {
		t.Fatalf("expected nil error for empty task list, got %v", err)
	}
}

func TestRun_Success(t *testing.T) {
	var aDone, bDone bool
	err := Run(context.Background(),
		func(context.Context) error {
			aDone = true
			return nil
		},
		nil,
		func(context.Context) error {
			bDone = true
			return nil
		},
	)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !aDone || !bDone {
		t.Fatalf("expected all non-nil tasks to run, a=%v b=%v", aDone, bDone)
	}
}

func TestRun_ErrorCancelsContext(t *testing.T) {
	blockedTaskCanceled := make(chan struct{}, 1)
	expectedErr := errors.New("boom")

	err := Run(context.Background(),
		func(context.Context) error {
			return expectedErr
		},
		func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				blockedTaskCanceled <- struct{}{}
				return nil
			case <-time.After(2 * time.Second):
				return errors.New("timed out waiting for cancellation")
			}
		},
	)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
	select {
	case <-blockedTaskCanceled:
	default:
		t.Fatal("expected second task to observe context cancellation")
	}
}
