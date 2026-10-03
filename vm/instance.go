package vm

import (
	"github.com/guoyyit/bytecode-vm-go/bytecode"
)

// slotSize is the memory footprint of a single operand stack or local slot.
const slotSize = 16

// Result is the outcome of a successful execution.
type Result struct {
	Value Value
	Steps uint64
}

// Instance is a single execution of a module. It holds all per-execution
// state and is not safe for concurrent use.
type Instance struct {
	module *bytecode.Module
	config Config

	frames []*Frame
	steps  uint64
	mem    int64

	// exc is non-nil when the VM is unwinding due to an exception.
	exc *Exception
	// finallyExc holds the exception to resume after a finally block.
	finallyExc *Exception

	// retValue holds the final return value of the entry function.
	retValue Value
	hasRet   bool
}

// Execute runs the named entry function with the given arguments.
func Execute(mod *bytecode.Module, cfg Config, entry string, args []Value) (*Result, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	entryFunc := findFunction(mod, entry)
	if entryFunc == nil {
		return nil, newRuntimeErrorf(ErrInvalidOperation, "entry function %q not found", entry)
	}
	if len(args) != int(entryFunc.NumParams) {
		return nil, newRuntimeErrorf(ErrInvalidOperation, "entry function %q expects %d args, got %d", entry, entryFunc.NumParams, len(args))
	}

	inst := &Instance{
		module: mod,
		config: cfg,
	}

	frame := newFrame(entryFunc, args)
	inst.frames = append(inst.frames, frame)
	inst.mem += int64(len(frame.locals)) * slotSize

	if err := inst.run(); err != nil {
		return nil, err
	}

	if len(inst.frames) != 0 {
		return nil, newRuntimeError(ErrInvalidOperation, "execution ended with frames still on the stack")
	}

	return &Result{Value: inst.retValue, Steps: inst.steps}, nil
}

func findFunction(mod *bytecode.Module, name string) *bytecode.Function {
	for _, f := range mod.Functions {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// topFrame returns the current top frame.
func (inst *Instance) topFrame() *Frame {
	return inst.frames[len(inst.frames)-1]
}

// popFrame removes the top frame and accounts for its memory.
func (inst *Instance) popFrame() {
	frame := inst.frames[len(inst.frames)-1]
	inst.frames = inst.frames[:len(inst.frames)-1]
	inst.mem -= int64(len(frame.locals)+len(frame.stack)) * slotSize
}

// pushValue pushes a value onto the current frame's stack, checking limits.
func (inst *Instance) pushValue(v Value) error {
	frame := inst.topFrame()
	if inst.config.MaxStackDepth > 0 && len(frame.stack) >= inst.config.MaxStackDepth {
		return newRuntimeErrorf(ErrStackOverflow, "stack overflow: depth %d exceeds limit %d", len(frame.stack), inst.config.MaxStackDepth)
	}
	inst.mem += slotSize
	if inst.config.MaxMemoryBytes > 0 && inst.mem > inst.config.MaxMemoryBytes {
		return newRuntimeErrorf(ErrMemoryLimit, "memory limit exceeded: %d bytes > %d", inst.mem, inst.config.MaxMemoryBytes)
	}
	frame.push(v)
	return nil
}

// popValue pops a value from the current frame's stack.
func (inst *Instance) popValue() (Value, error) {
	frame := inst.topFrame()
	if len(frame.stack) == 0 {
		return Value{}, newRuntimeError(ErrStackUnderflow, "stack underflow: pop from empty stack")
	}
	v := frame.pop()
	inst.mem -= slotSize
	return v, nil
}

// popN pops n values and returns them in order (first popped = last in slice).
func (inst *Instance) popN(n int) ([]Value, error) {
	frame := inst.topFrame()
	if len(frame.stack) < n {
		return nil, newRuntimeErrorf(ErrStackUnderflow, "stack underflow: need %d values, have %d", n, len(frame.stack))
	}
	out := make([]Value, n)
	for i := n - 1; i >= 0; i-- {
		out[i] = frame.pop()
		inst.mem -= slotSize
	}
	return out, nil
}

// allocHeap accounts for a heap allocation (string or exception).
func (inst *Instance) allocHeap(size int64) error {
	inst.mem += size
	if inst.config.MaxMemoryBytes > 0 && inst.mem > inst.config.MaxMemoryBytes {
		return newRuntimeErrorf(ErrMemoryLimit, "memory limit exceeded: %d bytes > %d", inst.mem, inst.config.MaxMemoryBytes)
	}
	return nil
}

// run is the main execution loop.
func (inst *Instance) run() error {
	for {
		// Check step limit.
		if inst.config.MaxSteps > 0 && inst.steps >= inst.config.MaxSteps {
			return newRuntimeErrorf(ErrStepLimit, "step limit exceeded: %d steps > %d", inst.steps, inst.config.MaxSteps)
		}

		// If unwinding due to an exception, handle it.
		if inst.exc != nil {
			exc := inst.exc
			if !inst.unwind() {
				re := newRuntimeErrorf(ErrUncaughtException, "uncaught exception: %s", exc.String())
				re.Trace = exc.Trace
				return re
			}
			continue
		}

		frame := inst.topFrame()
		if frame.pc >= len(frame.func_.Code) {
			return newRuntimeErrorf(ErrInvalidOperation, "execution ran past end of function %q", frame.func_.Name)
		}

		op := bytecode.Opcode(frame.func_.Code[frame.pc])
		inst.steps++
		frame.pc++

		if err := inst.exec(op, frame); err != nil {
			return err
		}

		// Returning from the entry function ends execution.
		if len(inst.frames) == 0 {
			break
		}
	}
	return nil
}

// unwind propagates an exception up the call stack. It returns true if the
// exception was caught, false if it propagated past the entry frame.
func (inst *Instance) unwind() bool {
	exc := inst.exc
	for len(inst.frames) > 0 {
		frame := inst.topFrame()
		for frame.tryDepth() > 0 {
			try, _ := frame.popTry()
			if try.handlerOffset >= 0 {
				// Caught: jump to handler with the exception on the stack.
				frame.pc = int(try.handlerOffset)
				_ = inst.pushValue(ExceptionValue(exc))
				inst.exc = nil
				return true
			}
			if try.finallyOffset >= 0 {
				// Run finally cleanup, then continue unwinding.
				inst.finallyExc = exc
				inst.exc = nil
				frame.pc = int(try.finallyOffset)
				return true
			}
		}
		// No handler in this frame; pop it and continue in the caller.
		inst.popFrame()
	}
	inst.exc = nil
	return false
}
