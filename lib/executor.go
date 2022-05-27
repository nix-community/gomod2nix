package lib

import (
	"sync"
)

// ParallellExecutor - Execute callback functions in parallell
type ParallellExecutor struct {
	errChan chan error
	wg      *sync.WaitGroup
	mux     *sync.Mutex
	guard   chan struct{}

	// Error returned by Wait(), cached for other Wait() invocations
	err  error
	done bool
}

func NewParallellExecutor(maxWorkers int) *ParallellExecutor {
	return &ParallellExecutor{
		errChan: make(chan error),
		mux:     new(sync.Mutex),
		wg:      new(sync.WaitGroup),
		guard:   make(chan struct{}, maxWorkers),

		err:  nil,
		done: false,
	}
}

func (e *ParallellExecutor) Add(fn func() error) {
	e.wg.Add(1)

	e.guard <- struct{}{} // Block

	go func() {
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

func (e *ParallellExecutor) Wait() error {
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
