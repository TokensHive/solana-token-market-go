package parallelx

import (
	"context"
	"sync"
)

type Task func(ctx context.Context) error

// Run executes tasks concurrently and returns the first error encountered.
// The shared context is canceled as soon as any task fails.
func Run(ctx context.Context, tasks ...Task) error {
	if len(tasks) == 0 {
		return nil
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, len(tasks))
	var wg sync.WaitGroup

	for _, task := range tasks {
		if task == nil {
			continue
		}
		wg.Add(1)
		go func(t Task) {
			defer wg.Done()
			if err := t(ctx); err != nil {
				select {
				case errCh <- err:
				default:
				}
				cancel()
			}
		}(task)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}
