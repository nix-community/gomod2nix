// Package lib provides utility functions and types for general use.
package lib

import (
	"sync"
)

// ParallelExecutor - Execute callback functions in parallel
type ParallelExecutor struct {
	errChan chan error
	wg      *sync.WaitGroup
	mux     *sync.Mutex
	guard   chan struct{}

	// Error returned by Wait(), cached for other Wait() invocations
	err  error
	done bool
}

func NewParallelExecutor(maxWorkers int) *ParallelExecutor {
	return &ParallelExecutor{
		errChan: make(chan error),
		mux:     new(sync.Mutex),
		wg:      new(sync.WaitGroup),
		guard:   make(chan struct{}, maxWorkers),

		err:  nil,
		done: false,
	}
}

func (e *ParallelExecutor) Add(fn func() error) {
	e.wg.Add(1)

	go func() {
		e.guard <- struct{}{} // Block until a worker is available
		defer e.wg.Done()
		defer func() {
			<-e.guard
		}()

		err := fn()
		if err != nil {
			e.errChan <- err
		}
	}()
}

func (e *ParallelExecutor) Wait() error {
	e.mux.Lock()
	defer e.mux.Unlock()

	if e.done {
		return e.err
	}

	var err error

	// Ensure channel is closed
	go func() {
		e.wg.Wait()
		close(e.errChan)
	}()

	for err = range e.errChan {
		if err != nil {
			break
		}
	}

	e.done = true
	e.err = err

	return err
}
