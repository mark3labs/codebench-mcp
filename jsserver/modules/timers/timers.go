package timers

import (
	"time"

	"github.com/grafana/sobek"
	"github.com/mark3labs/codebench-mcp/internal/logger"
	"github.com/mark3labs/codebench-mcp/jsserver/vm"
)

// TimersModule provides setTimeout, setInterval, clearTimeout, clearInterval
type TimersModule struct{}

// NewTimersModule creates a new timers module
func NewTimersModule() *TimersModule {
	return &TimersModule{}
}

// Name returns the module name
func (t *TimersModule) Name() string {
	return "timers"
}

// Setup initializes the timers module in the VM
func (t *TimersModule) Setup(runtime *sobek.Runtime, manager *vm.VMManager) error {
	logger.Debug("Setting up timers module")
	
	// setTimeout - copied exactly from ski
	runtime.Set("setTimeout", func(call sobek.FunctionCall) sobek.Value {
		logger.Debug("setTimeout called", "args", len(call.Arguments))
		
		callback, ok := sobek.AssertFunction(call.Argument(0))
		if !ok {
			panic(runtime.NewTypeError("setTimeout: first argument must be a function"))
		}

		i := call.Argument(1).ToInteger()
		if i < 1 || i > 2147483647 {
			i = 1
		}
		delay := time.Duration(i) * time.Millisecond
		logger.Debug("setTimeout delay", "ms", i)

		var args []sobek.Value
		if len(call.Arguments) > 2 {
			args = call.Arguments[2:]
		}

		logger.Debug("Getting enqueue function")
		enqueue := vm.EnqueueJob(runtime)
		logger.Debug("Creating timer")
		t := rtTimers(runtime).new(delay, false)
		logger.Debug("Timer created", "id", t.id)
		vm.Cleanup(runtime, t.stop)
		vm.AddPending(runtime) // Track this timer as a pending operation
		
		task := func() error {
			logger.Debug("Timer task executing", "id", t.id)
			defer t.stop()
			defer vm.RemovePending(runtime) // Remove pending operation when timer completes
			_, err := callback(sobek.Undefined(), args...)
			logger.Debug("Timer task completed", "id", t.id, "error", err)
			return err
		}

		logger.Debug("Starting timer goroutine", "id", t.id)
		go func() {
			logger.Debug("Timer goroutine started", "id", t.id)
			select {
			case <-t.timer:
				logger.Debug("Timer fired, enqueueing task", "id", t.id)
				enqueue(task)
				logger.Debug("Task enqueued", "id", t.id)
			case <-t.done:
				logger.Debug("Timer cancelled, enqueueing nothing", "id", t.id)
				vm.RemovePending(runtime) // Remove pending operation when timer is cancelled
				enqueue(nothing)
				logger.Debug("Nothing enqueued", "id", t.id)
			}
			logger.Debug("Timer goroutine finished", "id", t.id)
		}()

		logger.Debug("setTimeout returning", "id", t.id)
		return runtime.ToValue(t.id)
	})

	// clearTimeout - copied exactly from ski
	runtime.Set("clearTimeout", func(call sobek.FunctionCall) sobek.Value {
		id := call.Argument(0).ToInteger()
		logger.Debug("clearTimeout called", "id", id)
		rtTimers(runtime).stop(id)
		return sobek.Undefined()
	})

	// setInterval - copied exactly from ski
	runtime.Set("setInterval", func(call sobek.FunctionCall) sobek.Value {
		logger.Debug("setInterval called", "args", len(call.Arguments))
		
		callback, ok := sobek.AssertFunction(call.Argument(0))
		if !ok {
			panic(runtime.NewTypeError("setInterval: first argument must be a function"))
		}

		i := call.Argument(1).ToInteger()
		if i < 1 || i > 2147483647 {
			i = 1
		}
		delay := time.Duration(i) * time.Millisecond
		logger.Debug("setInterval delay", "ms", i)

		var args []sobek.Value
		if len(call.Arguments) > 2 {
			args = call.Arguments[2:]
		}

		enqueue := vm.EnqueueJob(runtime)
		t := rtTimers(runtime).new(delay, true)
		vm.Cleanup(runtime, t.stop)
		vm.AddPending(runtime) // Track this interval as a pending operation
		task := func() error { 
			logger.Debug("Interval task executing", "id", t.id)
			_, err := callback(sobek.Undefined(), args...)
			logger.Debug("Interval task completed", "id", t.id, "error", err)
			return err 
		}

		logger.Debug("Starting interval goroutine", "id", t.id)
		go func() {
			logger.Debug("Interval goroutine started", "id", t.id)
			for {
				select {
				case <-t.timer:
					logger.Debug("Interval fired, enqueueing task", "id", t.id)
					enqueue(task)
					logger.Debug("Interval task enqueued, getting new enqueue", "id", t.id)
					enqueue = vm.EnqueueJob(runtime)
				case <-t.done:
					logger.Debug("Interval cancelled, enqueueing nothing", "id", t.id)
					vm.RemovePending(runtime) // Remove pending operation when interval is cancelled
					enqueue(nothing)
					logger.Debug("Interval goroutine finished", "id", t.id)
					return
				}
			}
		}()

		return runtime.ToValue(t.id)
	})

	// clearInterval - copied exactly from ski
	runtime.Set("clearInterval", func(call sobek.FunctionCall) sobek.Value {
		id := call.Argument(0).ToInteger()
		logger.Debug("clearInterval called", "id", id)
		rtTimers(runtime).stop(id)
		return sobek.Undefined()
	})

	logger.Debug("Timers module setup complete")
	return nil
}

// Cleanup performs any necessary cleanup
func (t *TimersModule) Cleanup() error {
	// Cleanup is handled per-runtime via the symbol-based timers
	return nil
}

// IsEnabled checks if the module should be enabled based on configuration
func (t *TimersModule) IsEnabled(enabledModules map[string]bool) bool {
	enabled, exists := enabledModules["timers"]
	return exists && enabled
}

// timer represents a single timer instance (copied exactly from ski)
type timer struct {
	id      int64
	timer   <-chan time.Time
	done    chan struct{}
	cleanup func()
}

func (t *timer) stop() {
	logger.Debug("Stopping timer", "id", t.id)
	select {
	case <-t.done:
		// Channel already closed
		logger.Debug("Timer already stopped", "id", t.id)
		return
	default:
		// Channel not closed, close it
		close(t.done)
		t.cleanup()
		logger.Debug("Timer stopped", "id", t.id)
	}
}

// timers manages all timers for a runtime (copied exactly from ski)
type timers struct {
	id    int64
	timer map[int64]*timer
}

func (t *timers) new(delay time.Duration, repeat bool) *timer {
	t.id++
	id := t.id
	logger.Debug("Creating new timer", "id", id, "delay", delay, "repeat", repeat)
	
	n := &timer{
		id:   id,
		done: make(chan struct{}),
	}
	if repeat {
		t1 := time.NewTicker(delay)
		n.timer = t1.C
		n.cleanup = func() {
			logger.Debug("Cleaning up ticker", "id", id)
			delete(t.timer, id)
			t1.Stop()
		}
	} else {
		t1 := time.NewTimer(delay)
		n.timer = t1.C
		n.cleanup = func() {
			logger.Debug("Cleaning up timer", "id", id)
			delete(t.timer, id)
			t1.Stop()
		}
	}
	t.timer[id] = n
	logger.Debug("Timer created and stored", "id", id)
	return n
}

func (t *timers) stop(id int64) {
	logger.Debug("Stopping timer by ID", "id", id)
	if v, ok := t.timer[id]; ok {
		v.stop()
	} else {
		logger.Debug("Timer not found", "id", id)
	}
}

var symTimers = sobek.NewSymbol(`Symbol.__timers__`)

func rtTimers(rt *sobek.Runtime) *timers {
	global := rt.GlobalObject()
	v := global.GetSymbol(symTimers)
	if v == nil {
		logger.Debug("Creating new timers instance for runtime")
		t := &timers{timer: make(map[int64]*timer)}
		_ = global.SetSymbol(symTimers, t)
		return t
	}
	logger.Debug("Using existing timers instance")
	return v.Export().(*timers)
}

func nothing() error { 
	logger.Debug("Nothing function called")
	return nil 
}
