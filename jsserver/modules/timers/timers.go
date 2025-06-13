package timers

import (
	"time"

	"github.com/grafana/sobek"
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
	// setTimeout
	runtime.Set("setTimeout", func(call sobek.FunctionCall) sobek.Value {
		callback, ok := sobek.AssertFunction(call.Argument(0))
		if !ok {
			panic(runtime.NewTypeError("setTimeout: first argument must be a function"))
		}

		i := call.Argument(1).ToInteger()
		if i < 1 || i > 2147483647 {
			i = 1
		}
		delay := time.Duration(i) * time.Millisecond

		var args []sobek.Value
		if len(call.Arguments) > 2 {
			args = call.Arguments[2:]
		}

		enqueue := vm.EnqueueJob(runtime)
		t := rtTimers(runtime).new(delay, false)
		vm.Cleanup(runtime, t.stop)
		task := func() error {
			defer t.stop()
			_, err := callback(sobek.Undefined(), args...)
			return err
		}

		go func() {
			select {
			case <-t.timer:
				enqueue(task)
			case <-t.done:
				enqueue(nothing)
			}
		}()

		return runtime.ToValue(t.id)
	})

	// clearTimeout
	runtime.Set("clearTimeout", func(call sobek.FunctionCall) sobek.Value {
		id := call.Argument(0).ToInteger()
		rtTimers(runtime).stop(id)
		return sobek.Undefined()
	})

	// setInterval
	runtime.Set("setInterval", func(call sobek.FunctionCall) sobek.Value {
		callback, ok := sobek.AssertFunction(call.Argument(0))
		if !ok {
			panic(runtime.NewTypeError("setInterval: first argument must be a function"))
		}

		i := call.Argument(1).ToInteger()
		if i < 1 || i > 2147483647 {
			i = 1
		}
		delay := time.Duration(i) * time.Millisecond

		var args []sobek.Value
		if len(call.Arguments) > 2 {
			args = call.Arguments[2:]
		}

		enqueue := vm.EnqueueJob(runtime)
		t := rtTimers(runtime).new(delay, true)
		vm.Cleanup(runtime, t.stop)
		task := func() error { 
			_, err := callback(sobek.Undefined(), args...)
			return err 
		}

		go func() {
			for {
				select {
				case <-t.timer:
					enqueue(task)
					enqueue = vm.EnqueueJob(runtime)
				case <-t.done:
					enqueue(nothing)
					return
				}
			}
		}()

		return runtime.ToValue(t.id)
	})

	// clearInterval
	runtime.Set("clearInterval", func(call sobek.FunctionCall) sobek.Value {
		id := call.Argument(0).ToInteger()
		rtTimers(runtime).stop(id)
		return sobek.Undefined()
	})

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

// timer represents a single timer instance (copied from ski)
type timer struct {
	id      int64
	timer   <-chan time.Time
	done    chan struct{}
	cleanup func()
}

func (t *timer) stop() {
	select {
	case _, ok := <-t.done:
		if !ok {
			return
		}
	default:
	}
	close(t.done)
	t.cleanup()
}

// timers manages all timers for a runtime (copied from ski)
type timers struct {
	id    int64
	timer map[int64]*timer
}

func (t *timers) new(delay time.Duration, repeat bool) *timer {
	t.id++
	id := t.id
	n := &timer{
		id:   id,
		done: make(chan struct{}),
	}
	if repeat {
		t1 := time.NewTicker(delay)
		n.timer = t1.C
		n.cleanup = func() {
			delete(t.timer, id)
			t1.Stop()
		}
	} else {
		t1 := time.NewTimer(delay)
		n.timer = t1.C
		n.cleanup = func() {
			delete(t.timer, id)
			t1.Stop()
		}
	}
	t.timer[id] = n
	return n
}

func (t *timers) stop(id int64) {
	if v, ok := t.timer[id]; ok {
		v.stop()
	}
}

var symTimers = sobek.NewSymbol(`Symbol.__timers__`)

func rtTimers(rt *sobek.Runtime) *timers {
	global := rt.GlobalObject()
	v := global.GetSymbol(symTimers)
	if v == nil {
		t := &timers{timer: make(map[int64]*timer)}
		_ = global.SetSymbol(symTimers, t)
		return t
	}
	return v.Export().(*timers)
}

func nothing() error { return nil }
