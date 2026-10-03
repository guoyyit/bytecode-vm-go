package vm

import (
	"math"

	"github.com/guoyyit/bytecode-vm-go/bytecode"
)

// exec executes a single instruction. The frame's pc has already been
// advanced past the opcode byte.
func (inst *Instance) exec(op bytecode.Opcode, frame *Frame) error {
	code := frame.func_.Code

	switch op {
	case bytecode.OpNop:
		return nil

	case bytecode.OpPushInt:
		idx := readU32(code, frame.pc)
		frame.pc += 4
		return inst.pushValue(FromConstant(inst.module.Constants[idx]))

	case bytecode.OpPushFloat:
		idx := readU32(code, frame.pc)
		frame.pc += 4
		return inst.pushValue(FromConstant(inst.module.Constants[idx]))

	case bytecode.OpPushString:
		idx := readU32(code, frame.pc)
		frame.pc += 4
		return inst.pushValue(FromConstant(inst.module.Constants[idx]))

	case bytecode.OpPushTrue:
		return inst.pushValue(BoolValue(true))
	case bytecode.OpPushFalse:
		return inst.pushValue(BoolValue(false))
	case bytecode.OpPushNull:
		return inst.pushValue(NullValue())

	case bytecode.OpPop:
		_, err := inst.popValue()
		return err

	case bytecode.OpDup:
		v, err := inst.peekValue()
		if err != nil {
			return err
		}
		return inst.pushValue(v)

	case bytecode.OpAdd:
		return inst.execAdd()
	case bytecode.OpSub:
		return inst.execArith(func(a, b int64) int64 { return a - b }, func(a, b float64) float64 { return a - b })
	case bytecode.OpMul:
		return inst.execArith(func(a, b int64) int64 { return a * b }, func(a, b float64) float64 { return a * b })
	case bytecode.OpDiv:
		return inst.execDiv()
	case bytecode.OpMod:
		return inst.execMod()
	case bytecode.OpNeg:
		return inst.execNeg()

	case bytecode.OpEq:
		return inst.execCompare(func(c int) bool { return c == 0 })
	case bytecode.OpNe:
		return inst.execCompare(func(c int) bool { return c != 0 })
	case bytecode.OpLt:
		return inst.execCompare(func(c int) bool { return c < 0 })
	case bytecode.OpLe:
		return inst.execCompare(func(c int) bool { return c <= 0 })
	case bytecode.OpGt:
		return inst.execCompare(func(c int) bool { return c > 0 })
	case bytecode.OpGe:
		return inst.execCompare(func(c int) bool { return c >= 0 })

	case bytecode.OpAnd:
		return inst.execAnd()
	case bytecode.OpOr:
		return inst.execOr()
	case bytecode.OpNot:
		return inst.execNot()

	case bytecode.OpJmp:
		target := readI32(code, frame.pc)
		frame.pc += 4
		if err := inst.validateJumpTarget(frame, target); err != nil {
			return err
		}
		frame.pc = int(target)
		return nil

	case bytecode.OpJmpIfTrue:
		target := readI32(code, frame.pc)
		frame.pc += 4
		v, err := inst.popValue()
		if err != nil {
			return err
		}
		if v.IsTruthy() {
			if err := inst.validateJumpTarget(frame, target); err != nil {
				return err
			}
			frame.pc = int(target)
		}
		return nil

	case bytecode.OpJmpIfFalse:
		target := readI32(code, frame.pc)
		frame.pc += 4
		v, err := inst.popValue()
		if err != nil {
			return err
		}
		if !v.IsTruthy() {
			if err := inst.validateJumpTarget(frame, target); err != nil {
				return err
			}
			frame.pc = int(target)
		}
		return nil

	case bytecode.OpLoadLocal:
		idx := code[frame.pc]
		frame.pc++
		if int(idx) >= len(frame.locals) {
			return newRuntimeErrorf(ErrLocalIndex, "local index %d out of range (numLocals %d)", idx, len(frame.locals))
		}
		return inst.pushValue(frame.locals[idx])

	case bytecode.OpStoreLocal:
		idx := code[frame.pc]
		frame.pc++
		if int(idx) >= len(frame.locals) {
			return newRuntimeErrorf(ErrLocalIndex, "local index %d out of range (numLocals %d)", idx, len(frame.locals))
		}
		v, err := inst.popValue()
		if err != nil {
			return err
		}
		frame.locals[idx] = v
		return nil

	case bytecode.OpCall:
		funcIdx := readU32(code, frame.pc)
		argc := code[frame.pc+4]
		frame.pc += 5
		return inst.execCall(funcIdx, argc)

	case bytecode.OpRet:
		return inst.execRet()

	case bytecode.OpRetValue:
		return inst.execRetValue()

	case bytecode.OpThrow:
		return inst.execThrow()

	case bytecode.OpTry:
		handler := readI32(code, frame.pc)
		finally := readI32(code, frame.pc+4)
		frame.pc += 8
		frame.pushTry(handler, finally)
		return nil

	case bytecode.OpEndTry:
		frame.popTry()
		return nil

	case bytecode.OpEndFinally:
		if inst.finallyExc != nil {
			inst.exc = inst.finallyExc
			inst.finallyExc = nil
		}
		return nil

	case bytecode.OpPow:
		return inst.execPow()

	default:
		return newRuntimeErrorf(ErrInvalidOperation, "unknown opcode 0x%02x", op)
	}
}

// peekValue returns the top value without popping it.
func (inst *Instance) peekValue() (Value, error) {
	frame := inst.topFrame()
	if len(frame.stack) == 0 {
		return Value{}, newRuntimeError(ErrStackUnderflow, "stack underflow: peek from empty stack")
	}
	return frame.peek(), nil
}

func (inst *Instance) validateJumpTarget(frame *Frame, target int32) error {
	if target < 0 || int(target) >= len(frame.func_.Code) {
		return newRuntimeErrorf(ErrInvalidJumpTarget, "jump target %d out of range [0,%d)", target, len(frame.func_.Code))
	}
	if !frame.func_.ValidInstStart(int(target)) {
		return newRuntimeErrorf(ErrInvalidJumpTarget, "jump target %d is not an instruction boundary", target)
	}
	return nil
}

func (inst *Instance) execAdd() error {
	b, err := inst.popValue()
	if err != nil {
		return err
	}
	a, err := inst.popValue()
	if err != nil {
		return err
	}
	// String concatenation.
	if a.Kind == KString || b.Kind == KString {
		sa, err := valueToString(a)
		if err != nil {
			return err
		}
		sb, err := valueToString(b)
		if err != nil {
			return err
		}
		s := sa + sb
		if err := inst.allocHeap(16 + int64(len(s))); err != nil {
			return err
		}
		return inst.pushValue(StringValue(s))
	}
	if a.Kind == KInt && b.Kind == KInt {
		return inst.pushValue(IntValue(a.Int + b.Int))
	}
	af, err := toFloat(a)
	if err != nil {
		return err
	}
	bf, err := toFloat(b)
	if err != nil {
		return err
	}
	return inst.pushValue(FloatValue(af + bf))
}

func (inst *Instance) execArith(iop func(int64, int64) int64, fop func(float64, float64) float64) error {
	b, err := inst.popValue()
	if err != nil {
		return err
	}
	a, err := inst.popValue()
	if err != nil {
		return err
	}
	if a.Kind == KInt && b.Kind == KInt {
		return inst.pushValue(IntValue(iop(a.Int, b.Int)))
	}
	af, err := toFloat(a)
	if err != nil {
		return err
	}
	bf, err := toFloat(b)
	if err != nil {
		return err
	}
	return inst.pushValue(FloatValue(fop(af, bf)))
}

func (inst *Instance) execDiv() error {
	b, err := inst.popValue()
	if err != nil {
		return err
	}
	a, err := inst.popValue()
	if err != nil {
		return err
	}
	if a.Kind == KInt && b.Kind == KInt {
		if b.Int == 0 {
			return newRuntimeError(ErrDivisionByZero, "division by zero")
		}
		return inst.pushValue(IntValue(a.Int / b.Int))
	}
	af, err := toFloat(a)
	if err != nil {
		return err
	}
	bf, err := toFloat(b)
	if err != nil {
		return err
	}
	if bf == 0 {
		return newRuntimeError(ErrDivisionByZero, "division by zero")
	}
	return inst.pushValue(FloatValue(af / bf))
}

func (inst *Instance) execMod() error {
	b, err := inst.popValue()
	if err != nil {
		return err
	}
	a, err := inst.popValue()
	if err != nil {
		return err
	}
	if a.Kind == KInt && b.Kind == KInt {
		if b.Int == 0 {
			return newRuntimeError(ErrDivisionByZero, "modulo by zero")
		}
		return inst.pushValue(IntValue(a.Int % b.Int))
	}
	af, err := toFloat(a)
	if err != nil {
		return err
	}
	bf, err := toFloat(b)
	if err != nil {
		return err
	}
	if bf == 0 {
		return newRuntimeError(ErrDivisionByZero, "modulo by zero")
	}
	return inst.pushValue(FloatValue(math.Mod(af, bf)))
}

func (inst *Instance) execNeg() error {
	v, err := inst.popValue()
	if err != nil {
		return err
	}
	switch v.Kind {
	case KInt:
		return inst.pushValue(IntValue(-v.Int))
	case KFloat:
		return inst.pushValue(FloatValue(-v.Float))
	}
	return newRuntimeErrorf(ErrTypeMismatch, "negate requires number, got %s", v.Kind)
}

func (inst *Instance) execCompare(cmp func(int) bool) error {
	b, err := inst.popValue()
	if err != nil {
		return err
	}
	a, err := inst.popValue()
	if err != nil {
		return err
	}
	// Equality across different kinds is always false (except null).
	if a.Kind != b.Kind {
		if a.Kind == KNull || b.Kind == KNull {
			return inst.pushValue(BoolValue(cmp(compareForEquality(a, b))))
		}
		// For ordering comparisons across kinds, this is a type error.
		if cmp(0) { // EQ or NE path
			return inst.pushValue(BoolValue(cmp(1))) // not equal
		}
		return newRuntimeErrorf(ErrTypeMismatch, "cannot compare %s and %s for ordering", a.Kind, b.Kind)
	}
	c, err := compareValues(a, b)
	if err != nil {
		return err
	}
	return inst.pushValue(BoolValue(cmp(c)))
}

// compareForEquality returns 0 if a and b are equal, non-zero otherwise.
func compareForEquality(a, b Value) int {
	if a.Kind == b.Kind {
		c, _ := compareValues(a, b)
		return c
	}
	return 1
}

func (inst *Instance) execAnd() error {
	b, err := inst.popValue()
	if err != nil {
		return err
	}
	a, err := inst.popValue()
	if err != nil {
		return err
	}
	if a.Kind == KInt && b.Kind == KInt {
		return inst.pushValue(IntValue(a.Int & b.Int))
	}
	return inst.pushValue(BoolValue(a.IsTruthy() && b.IsTruthy()))
}

func (inst *Instance) execOr() error {
	b, err := inst.popValue()
	if err != nil {
		return err
	}
	a, err := inst.popValue()
	if err != nil {
		return err
	}
	if a.Kind == KInt && b.Kind == KInt {
		return inst.pushValue(IntValue(a.Int | b.Int))
	}
	return inst.pushValue(BoolValue(a.IsTruthy() || b.IsTruthy()))
}

func (inst *Instance) execNot() error {
	v, err := inst.popValue()
	if err != nil {
		return err
	}
	if v.Kind == KInt {
		return inst.pushValue(IntValue(^v.Int))
	}
	return inst.pushValue(BoolValue(!v.IsTruthy()))
}

func (inst *Instance) execCall(funcIdx uint32, argc uint8) error {
	if inst.config.MaxCallDepth > 0 && len(inst.frames) >= inst.config.MaxCallDepth {
		return newRuntimeErrorf(ErrCallDepthLimit, "call depth limit exceeded: %d >= %d", len(inst.frames), inst.config.MaxCallDepth)
	}
	target := inst.module.Functions[funcIdx]
	if int(argc) > int(target.NumParams) {
		return newRuntimeErrorf(ErrInvalidOperation, "call to %q with %d args but function takes %d", target.Name, argc, target.NumParams)
	}
	args, err := inst.popN(int(argc))
	if err != nil {
		return err
	}
	newFrame := newFrame(target, args)
	inst.frames = append(inst.frames, newFrame)
	inst.mem += int64(len(newFrame.locals)) * slotSize
	return nil
}

func (inst *Instance) execRet() error {
	inst.popFrame()
	if len(inst.frames) == 0 {
		inst.hasRet = true
		inst.retValue = NullValue()
		return nil
	}
	return inst.pushValue(NullValue())
}

func (inst *Instance) execRetValue() error {
	v, err := inst.popValue()
	if err != nil {
		return err
	}
	inst.popFrame()
	if len(inst.frames) == 0 {
		inst.hasRet = true
		inst.retValue = v
		return nil
	}
	return inst.pushValue(v)
}

func (inst *Instance) execThrow() error {
	v, err := inst.popValue()
	if err != nil {
		return err
	}
	var exc *Exception
	if v.Kind == KException && v.Exc != nil {
		exc = v.Exc
	} else {
		exc = NewException(v.String())
		if err := inst.allocHeap(exc.memSize()); err != nil {
			return err
		}
	}
	exc.Trace = inst.buildTrace()
	inst.exc = exc
	return nil
}

func (inst *Instance) execPow() error {
	exp, err := inst.popValue()
	if err != nil {
		return err
	}
	base, err := inst.popValue()
	if err != nil {
		return err
	}
	if base.Kind == KInt && exp.Kind == KInt {
		result := int64(1)
		b := base.Int
		e := exp.Int
		if e < 0 {
			bf := float64(base.Int)
			return inst.pushValue(FloatValue(math.Pow(bf, float64(e))))
		}
		for e > 0 {
			if e&1 == 1 {
				result *= b
			}
			b *= b
			e >>= 1
		}
		return inst.pushValue(IntValue(result))
	}
	bf, err := toFloat(base)
	if err != nil {
		return err
	}
	ef, err := toFloat(exp)
	if err != nil {
		return err
	}
	return inst.pushValue(FloatValue(math.Pow(bf, ef)))
}

// buildTrace constructs a call trace from the current frames.
func (inst *Instance) buildTrace() []FrameInfo {
	trace := make([]FrameInfo, 0, len(inst.frames))
	for _, f := range inst.frames {
		trace = append(trace, FrameInfo{Function: f.func_.Name, Offset: f.pc})
	}
	return trace
}

// --- helpers ---

func readU32(code []byte, pos int) uint32 {
	return uint32(code[pos]) | uint32(code[pos+1])<<8 | uint32(code[pos+2])<<16 | uint32(code[pos+3])<<24
}

func readI32(code []byte, pos int) int32 {
	return int32(readU32(code, pos))
}

func toFloat(v Value) (float64, error) {
	switch v.Kind {
	case KInt:
		return float64(v.Int), nil
	case KFloat:
		return v.Float, nil
	}
	return 0, newRuntimeErrorf(ErrTypeMismatch, "expected number, got %s", v.Kind)
}

func valueToString(v Value) (string, error) {
	switch v.Kind {
	case KString:
		return v.Str, nil
	case KInt, KFloat, KBool, KNull:
		return v.String(), nil
	case KException:
		return v.String(), nil
	}
	return "", newRuntimeErrorf(ErrTypeMismatch, "cannot convert %s to string", v.Kind)
}

// compareValues compares two values of the same or numeric kinds.
// Returns -1, 0, or 1. Returns an error if the kinds are not comparable.
func compareValues(a, b Value) (int, error) {
	// Same kind comparisons.
	switch a.Kind {
	case KInt:
		if b.Kind == KInt {
			if a.Int < b.Int {
				return -1, nil
			} else if a.Int > b.Int {
				return 1, nil
			}
			return 0, nil
		}
	case KFloat:
		if b.Kind == KFloat {
			if a.Float < b.Float {
				return -1, nil
			} else if a.Float > b.Float {
				return 1, nil
			}
			return 0, nil
		}
	case KString:
		if b.Kind == KString {
			if a.Str < b.Str {
				return -1, nil
			} else if a.Str > b.Str {
				return 1, nil
			}
			return 0, nil
		}
	case KBool:
		if b.Kind == KBool {
			if a.Bool == b.Bool {
				return 0, nil
			}
			if !a.Bool {
				return -1, nil
			}
			return 1, nil
		}
	case KNull:
		if b.Kind == KNull {
			return 0, nil
		}
	}

	// Numeric cross-type comparison.
	if (a.Kind == KInt || a.Kind == KFloat) && (b.Kind == KInt || b.Kind == KFloat) {
		af, _ := toFloat(a)
		bf, _ := toFloat(b)
		if af < bf {
			return -1, nil
		} else if af > bf {
			return 1, nil
		}
		return 0, nil
	}

	return 0, newRuntimeErrorf(ErrTypeMismatch, "cannot compare %s and %s", a.Kind, b.Kind)
}
