package vm

import "github.com/guoyyit/bytecode-vm-go/bytecode"

// Frame is a single call frame.
type Frame struct {
	func_   *bytecode.Function
	pc      int
	locals  []Value
	stack   []Value
	tryRecs []tryRecord
}

func newFrame(f *bytecode.Function, args []Value) *Frame {
	locals := make([]Value, f.NumLocals)
	for i := 0; i < int(f.NumParams) && i < len(args); i++ {
		locals[i] = args[i]
	}
	return &Frame{
		func_:  f,
		pc:     0,
		locals: locals,
		stack:  make([]Value, 0, 16),
	}
}

// Func returns the function of this frame.
func (f *Frame) Func() *bytecode.Function { return f.func_ }

// PC returns the current program counter.
func (f *Frame) PC() int { return f.pc }

// StackLen returns the current operand stack depth.
func (f *Frame) StackLen() int { return len(f.stack) }

// Locals returns a copy of the locals.
func (f *Frame) Locals() []Value {
	out := make([]Value, len(f.locals))
	copy(out, f.locals)
	return out
}

// push pushes a value onto the operand stack.
func (f *Frame) push(v Value) {
	f.stack = append(f.stack, v)
}

// pop pops a value from the operand stack.
func (f *Frame) pop() Value {
	v := f.stack[len(f.stack)-1]
	f.stack = f.stack[:len(f.stack)-1]
	return v
}

// peek returns the top value without popping.
func (f *Frame) peek() Value {
	return f.stack[len(f.stack)-1]
}

// pushTry registers a try record.
func (f *Frame) pushTry(handler, finally int32) {
	f.tryRecs = append(f.tryRecs, tryRecord{handlerOffset: handler, finallyOffset: finally})
}

// popTry removes and returns the top try record.
func (f *Frame) popTry() (tryRecord, bool) {
	if len(f.tryRecs) == 0 {
		return tryRecord{}, false
	}
	t := f.tryRecs[len(f.tryRecs)-1]
	f.tryRecs = f.tryRecs[:len(f.tryRecs)-1]
	return t, true
}

// tryDepth returns the number of active try records.
func (f *Frame) tryDepth() int { return len(f.tryRecs) }
