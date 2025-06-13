package vm

import (
	"sync"

	"github.com/grafana/sobek"
	"github.com/mark3labs/codebench-mcp/internal/logger"
)

// EventLoop implements an event loop for asynchronous JavaScript operations
type EventLoop struct {
	queue   []func() error // queue to store the job to be executed
	cleanup []func()       // job of cleanup
	enqueue uint           // Count of job in the event loop
	pending uint           // Count of pending async operations (timers, etc.)
	cond    *sync.Cond     // Condition variable for synchronization
}

// NewEventLoop creates a new EventLoop instance
func NewEventLoop() *EventLoop {
	return &EventLoop{
		cond:    sync.NewCond(new(sync.Mutex)),
		cleanup: make([]func(), 0),
	}
}

// Start the event loop and execute the provided function
func (e *EventLoop) Start(task func() error) (err error) {
	e.cond.L.Lock()
	e.queue = []func() error{task}
	e.cond.L.Unlock()
	
	for {
		e.cond.L.Lock()

		if len(e.queue) > 0 {
			queue := e.queue
			e.queue = make([]func() error, 0, len(queue))
			e.cond.L.Unlock()

			for _, job := range queue {
				if err2 := job(); err2 != nil {
					if err != nil {
						err = append(err.(joinError), err2)
					} else {
						err = joinError{err2}
					}
				}
			}
			continue
		}

		if e.enqueue > 0 || e.pending > 0 {
			e.cond.Wait()
			e.cond.L.Unlock()
			continue
		}

		if len(e.cleanup) > 0 {
			cleanup := e.cleanup
			e.cleanup = e.cleanup[:0]
			e.cond.L.Unlock()

			for _, clean := range cleanup {
				clean()
			}
		} else {
			e.cond.L.Unlock()
		}

		return
	}
}

// Enqueue add a job to the job queue.
type Enqueue func(func() error)

// EnqueueJob return a function Enqueue to add a job to the job queue.
func (e *EventLoop) EnqueueJob() Enqueue {
	e.cond.L.Lock()
	called := false
	e.enqueue++
	e.cond.L.Unlock()
	return func(job func() error) {
		e.cond.L.Lock()
		defer e.cond.L.Unlock()
		switch {
		case called:
			panic("Enqueue already called")
		case e.enqueue == 0:
			return // Eventloop stopped
		}
		e.queue = append(e.queue, job) // Add the job to the queue
		called = true
		e.enqueue--
		e.cond.Signal() // Signal the condition variable
	}
}

// Stop the eventloop with the provided error
func (e *EventLoop) Stop(err error) {
	e.cond.L.Lock()
	defer e.cond.L.Unlock()
	// clean the queue
	e.queue = append(e.queue[:0], func() error { return err })
	e.enqueue = 0
	e.cond.Signal()
}

// Cleanup add a function to execute when run finish.
func (e *EventLoop) Cleanup(job ...func()) {
	e.cond.L.Lock()
	defer e.cond.L.Unlock()

	e.cleanup = append(e.cleanup, job...)
}

// joinError represents multiple errors joined together
type joinError []error

func (je joinError) Error() string {
	if len(je) == 0 {
		return ""
	}
	if len(je) == 1 {
		return je[0].Error()
	}
	
	result := je[0].Error()
	for _, err := range je[1:] {
		result += "; " + err.Error()
	}
	return result
}

// AddPending increments the pending operation counter
func (e *EventLoop) AddPending() {
	e.cond.L.Lock()
	defer e.cond.L.Unlock()
	e.pending++
	logger.Debug("Added pending operation", "pending", e.pending)
}

// RemovePending decrements the pending operation counter
func (e *EventLoop) RemovePending() {
	e.cond.L.Lock()
	defer e.cond.L.Unlock()
	if e.pending > 0 {
		e.pending--
	}
	logger.Debug("Removed pending operation", "pending", e.pending)
	e.cond.Signal()
}

// Helper functions for runtime integration

var symbolVM = sobek.NewSymbol("Symbol.__vm__")

// vmSelf holds a reference to the VM for runtime access
type vmSelf struct {
	vm *VM
}

// EnqueueJob returns a function to enqueue jobs for the given runtime
func EnqueueJob(rt *sobek.Runtime) Enqueue {
	return getVMFromRuntime(rt).eventLoop.EnqueueJob()
}

// Cleanup adds cleanup functions for the given runtime
func Cleanup(rt *sobek.Runtime, job ...func()) {
	getVMFromRuntime(rt).eventLoop.Cleanup(job...)
}

// AddPending adds a pending operation for the given runtime
func AddPending(rt *sobek.Runtime) {
	getVMFromRuntime(rt).eventLoop.AddPending()
}

// RemovePending removes a pending operation for the given runtime
func RemovePending(rt *sobek.Runtime) {
	getVMFromRuntime(rt).eventLoop.RemovePending()
}

// getVMFromRuntime extracts the VM instance from the runtime
func getVMFromRuntime(rt *sobek.Runtime) *VM {
	value := rt.GlobalObject().GetSymbol(symbolVM)
	if value != nil {
		if vmSelf, ok := value.Export().(*vmSelf); ok {
			return vmSelf.vm
		}
	}
	panic(rt.NewTypeError("VM symbol not found in runtime - this shouldn't happen"))
}