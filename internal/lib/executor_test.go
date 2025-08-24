package lib

import (
	"errors"
	"testing"
	"time"
)

// TestParallelExecutor_fnAlwaysErrors ensures that the executor does not block
// forever when there are more erroring functions than workers. This is a
// regression test.
func TestParallelExecutor_fnAlwaysErrors(t *testing.T) {
	const maxWorkers = 1
	executor := NewParallelExecutor(1)

	for range maxWorkers + 1 {
		executor.Add(func() error {
			return errors.New("testerror")
		})
	}

	errCh := make(chan error)
	go func() {
		defer close(errCh)
		errCh <- executor.Wait()
	}()

	select {
	case err := <-errCh:
		if err == nil {
			t.Error("Expected error, got nil")
		}
	case <-time.After(10 * time.Second):
		t.Error("Timed out waiting for executor to finish: deadlock")
	}
}
