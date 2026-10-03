package vm

import (
	"encoding/binary"
	"fmt"
)

// 默认资源上限（Config 字段为零时采用）。
const (
	defaultMaxSteps     = 1_000_000
	defaultMaxMemory    = 1 << 20 // 槽位
	defaultMaxCallDepth = 256
	defaultMaxStack     = 1024
)

// 内存记账单位（槽位）。
const (
	memPerFrame   = 8 // 每帧固定开销
	memPerHandler = 1
)

// Config 控制单次执行的资源上限。零值字段使用默认值。
type Config struct {
	MaxSteps     uint64 // 指令步数上限
	MaxMemory    int    // 内存上限（单位：槽位，含操作数栈、局部变量、帧与异常处理器开销）
	MaxCallDepth int    // 调用深度上限
	MaxStack     int    // 单帧操作数栈上限
}

func (c Config) withDefaults() Config {
	if c.MaxSteps == 0 {
		c.MaxSteps = defaultMaxSteps
	}
	if c.MaxMemory == 0 {
		c.MaxMemory = defaultMaxMemory
	}
	if c.MaxCallDepth == 0 {
		c.MaxCallDepth = defaultMaxCallDepth
	}
	if c.MaxStack == 0 {
		c.MaxStack = defaultMaxStack
	}
	return c
}

// Stats 是单次执行的统计指标，固定配置与输入下结果确定。
type Stats struct {
	Instructions uint64 // 已执行指令数
	PeakStack    int    // 全局操作数栈深度峰值（各帧合计）
	PeakMemory   int    // 内存占用峰值（槽位）
	Calls        uint64 // 函数调用次数
}

// handler 是一个已注册的异常块。
type handler struct {
	catchPC  int // 处理逻辑入口地址
	stackLen int // 注册时的操作数栈深度（展开时恢复）
}

// frame 是一次函数调用的运行状态。
type frame struct {
	fn       *Function
	locals   []Value
	stack    []Value
	pc       int
	handlers []handler
}

// VM 是单次执行的实例，状态与其他实例完全隔离。
type VM struct {
	prog    *Program
	cfg     Config
	stats   Stats
	frames  []frame
	memUsed int
}

// New 创建一个执行实例。Program 在实例间共享且只读，VM 不持有任何共享可变状态。
func New(p *Program, cfg Config) *VM {
	return &VM{prog: p, cfg: cfg.withDefaults()}
}

// Run 从入口函数开始执行，返回结果值与统计指标。
// 每次 Run 都从干净状态开始；出错时不保留任何半执行的可见状态。
func (m *VM) Run(args ...Value) (Value, Stats, error) {
	m.stats = Stats{}
	m.frames = m.frames[:0]
	m.memUsed = 0

	entry := &m.prog.Funcs[m.prog.Entry]
	if len(args) != entry.NumParams {
		return Value{}, m.stats, m.err(KindBadCall,
			fmt.Sprintf("entry %q expects %d args, got %d", entry.Name, entry.NumParams, len(args)))
	}
	if err := m.pushFrame(entry, args); err != nil {
		return Value{}, m.stats, err
	}
	v, err := m.loop()
	return v, m.stats, err
}

// err 构造携带当前调用轨迹的运行时错误。
func (m *VM) err(kind RuntimeKind, msg string) *RuntimeError {
	trace := make([]FrameInfo, 0, len(m.frames))
	for i := len(m.frames) - 1; i >= 0; i-- {
		trace = append(trace, FrameInfo{Function: m.frames[i].fn.Name, PC: m.frames[i].pc})
	}
	return &RuntimeError{Kind: kind, Message: msg, Trace: trace}
}

// grow 记账 n 个槽位，超出内存上限时报错。
func (m *VM) grow(n int) error {
	if m.memUsed+n > m.cfg.MaxMemory {
		return m.err(KindMemoryLimitExceeded,
			fmt.Sprintf("need %d slots, used %d, limit %d", n, m.memUsed, m.cfg.MaxMemory))
	}
	m.memUsed += n
	if m.memUsed > m.stats.PeakMemory {
		m.stats.PeakMemory = m.memUsed
	}
	return nil
}

func (m *VM) shrink(n int) { m.memUsed -= n }

// pushFrame 压入新调用帧并记账。
func (m *VM) pushFrame(fn *Function, args []Value) error {
	if len(m.frames) >= m.cfg.MaxCallDepth {
		return m.err(KindCallDepthExceeded,
			fmt.Sprintf("call depth %d exceeds limit %d", len(m.frames)+1, m.cfg.MaxCallDepth))
	}
	need := memPerFrame + fn.NumLocals
	if err := m.grow(need); err != nil {
		return err
	}
	locals := make([]Value, fn.NumLocals)
	copy(locals, args)
	m.frames = append(m.frames, frame{fn: fn, locals: locals})
	m.stats.Calls++
	return nil
}

// popFrame 弹出帧并释放其内存记账。
func (m *VM) popFrame(f *frame) {
	m.shrink(memPerFrame + len(f.locals) + len(f.stack) + len(f.handlers)*memPerHandler)
	m.frames = m.frames[:len(m.frames)-1]
}

// push 向当前帧操作数栈压值，检查栈上限与内存上限。
func (m *VM) push(f *frame, v Value) error {
	if len(f.stack) >= m.cfg.MaxStack {
		return m.err(KindStackOverflow,
			fmt.Sprintf("operand stack depth %d exceeds limit %d", len(f.stack)+1, m.cfg.MaxStack))
	}
	if err := m.grow(1); err != nil {
		return err
	}
	f.stack = append(f.stack, v)
	total := 0
	for i := range m.frames {
		total += len(m.frames[i].stack)
	}
	if total > m.stats.PeakStack {
		m.stats.PeakStack = total
	}
	return nil
}

// pop 从当前帧操作数栈弹值，下溢时报错。
func (m *VM) pop(f *frame) (Value, error) {
	if len(f.stack) == 0 {
		return Value{}, m.err(KindStackUnderflow, "pop from empty operand stack")
	}
	v := f.stack[len(f.stack)-1]
	f.stack = f.stack[:len(f.stack)-1]
	m.shrink(1)
	return v, nil
}

// popInt 弹出整数值，类型不符时报错。
func (m *VM) popInt(f *frame, op string) (int64, error) {
	v, err := m.pop(f)
	if err != nil {
		return 0, err
	}
	if v.Kind != KindInt {
		return 0, m.err(KindTypeError, fmt.Sprintf("%s expects int, got %s", op, v))
	}
	return v.I, nil
}

// loop 是解释器主循环。
func (m *VM) loop() (Value, error) {
	for {
		f := &m.frames[len(m.frames)-1]
		if f.pc >= len(f.fn.Code) {
			return Value{}, m.err(KindBadJump,
				fmt.Sprintf("pc=%d fell off end of function %q", f.pc, f.fn.Name))
		}
		if m.stats.Instructions >= m.cfg.MaxSteps {
			return Value{}, m.err(KindStepLimitExceeded,
				fmt.Sprintf("step limit %d reached", m.cfg.MaxSteps))
		}
		m.stats.Instructions++

		pc := f.pc
		op := Op(f.fn.Code[pc])
		info := opTable[byte(op)]
		if info == nil { // 加载期已校验，此处为防御性检查
			return Value{}, m.err(KindBadJump, fmt.Sprintf("unknown opcode %#02x at pc=%d", byte(op), pc))
		}
		f.pc += 1 + info.operandN

		u16op := func() uint16 { return binary.LittleEndian.Uint16(f.fn.Code[pc+1:]) }
		u32op := func() uint32 { return binary.LittleEndian.Uint32(f.fn.Code[pc+1:]) }

		switch op {
		case OpNop:
		case OpConst:
			c := m.prog.Consts[int(u16op())]
			if err := m.push(f, Value{Kind: c.Kind, I: c.I, S: c.S}); err != nil {
				return Value{}, err
			}
		case OpLoad:
			slot := int(u16op())
			if slot >= len(f.locals) {
				return Value{}, m.err(KindInvalidLocal,
					fmt.Sprintf("load slot %d out of %d locals", slot, len(f.locals)))
			}
			if err := m.push(f, f.locals[slot]); err != nil {
				return Value{}, err
			}
		case OpStore:
			slot := int(u16op())
			if slot >= len(f.locals) {
				return Value{}, m.err(KindInvalidLocal,
					fmt.Sprintf("store slot %d out of %d locals", slot, len(f.locals)))
			}
			v, err := m.pop(f)
			if err != nil {
				return Value{}, err
			}
			f.locals[slot] = v
		case OpPop:
			if _, err := m.pop(f); err != nil {
				return Value{}, err
			}
		case OpDup:
			if len(f.stack) == 0 {
				return Value{}, m.err(KindStackUnderflow, "dup on empty operand stack")
			}
			if err := m.push(f, f.stack[len(f.stack)-1]); err != nil {
				return Value{}, err
			}

		case OpAdd, OpSub, OpMul, OpDiv, OpMod:
			b, err := m.popInt(f, info.name)
			if err != nil {
				return Value{}, err
			}
			a, err := m.popInt(f, info.name)
			if err != nil {
				return Value{}, err
			}
			var r int64
			switch op {
			case OpAdd:
				r = a + b
			case OpSub:
				r = a - b
			case OpMul:
				r = a * b
			case OpDiv:
				if b == 0 {
					return Value{}, m.err(KindDivByZero, "integer division by zero")
				}
				r = a / b
			case OpMod:
				if b == 0 {
					return Value{}, m.err(KindDivByZero, "integer modulo by zero")
				}
				r = a % b
			}
			if err := m.push(f, Int(r)); err != nil {
				return Value{}, err
			}
		case OpNeg:
			a, err := m.popInt(f, "neg")
			if err != nil {
				return Value{}, err
			}
			if err := m.push(f, Int(-a)); err != nil {
				return Value{}, err
			}

		case OpAnd, OpOr, OpXor, OpShl, OpShr:
			b, err := m.popInt(f, info.name)
			if err != nil {
				return Value{}, err
			}
			a, err := m.popInt(f, info.name)
			if err != nil {
				return Value{}, err
			}
			var r int64
			switch op {
			case OpAnd:
				r = a & b
			case OpOr:
				r = a | b
			case OpXor:
				r = a ^ b
			case OpShl:
				r = a << uint(b&63)
			case OpShr:
				r = a >> uint(b&63)
			}
			if err := m.push(f, Int(r)); err != nil {
				return Value{}, err
			}
		case OpNot:
			a, err := m.popInt(f, "not")
			if err != nil {
				return Value{}, err
			}
			if err := m.push(f, Int(^a)); err != nil {
				return Value{}, err
			}

		case OpEq, OpNe:
			b, err := m.pop(f)
			if err != nil {
				return Value{}, err
			}
			a, err := m.pop(f)
			if err != nil {
				return Value{}, err
			}
			eq := a.Kind == b.Kind && a.I == b.I && a.S == b.S
			if op == OpNe {
				eq = !eq
			}
			if err := m.push(f, boolInt(eq)); err != nil {
				return Value{}, err
			}
		case OpLt, OpLe, OpGt, OpGe:
			b, err := m.popInt(f, info.name)
			if err != nil {
				return Value{}, err
			}
			a, err := m.popInt(f, info.name)
			if err != nil {
				return Value{}, err
			}
			var r bool
			switch op {
			case OpLt:
				r = a < b
			case OpLe:
				r = a <= b
			case OpGt:
				r = a > b
			case OpGe:
				r = a >= b
			}
			if err := m.push(f, boolInt(r)); err != nil {
				return Value{}, err
			}

		case OpJmp, OpJmpIfFalse, OpJmpIfTrue:
			target := int(u32op())
			take := true
			if op != OpJmp {
				c, err := m.pop(f)
				if err != nil {
					return Value{}, err
				}
				take = c.truthy() == (op == OpJmpIfTrue)
			}
			if take {
				if target >= len(f.fn.Code) {
					return Value{}, m.err(KindBadJump,
						fmt.Sprintf("jump target %d out of code size %d", target, len(f.fn.Code)))
				}
				f.pc = target
			}

		case OpCall:
			callee := &m.prog.Funcs[int(u16op())]
			args := make([]Value, callee.NumParams)
			for i := callee.NumParams - 1; i >= 0; i-- {
				v, err := m.pop(f)
				if err != nil {
					return Value{}, err
				}
				args[i] = v
			}
			if err := m.pushFrame(callee, args); err != nil {
				return Value{}, err
			}
		case OpRet:
			var ret Value
			if len(f.stack) > 0 {
				v, err := m.pop(f)
				if err != nil {
					return Value{}, err
				}
				ret = v
			}
			m.popFrame(f)
			if len(m.frames) == 0 {
				return ret, nil
			}
			if err := m.push(&m.frames[len(m.frames)-1], ret); err != nil {
				return Value{}, err
			}

		case OpTry:
			if err := m.grow(memPerHandler); err != nil {
				return Value{}, err
			}
			f.handlers = append(f.handlers, handler{catchPC: int(u32op()), stackLen: len(f.stack)})
		case OpTryEnd:
			if len(f.handlers) == 0 {
				return Value{}, m.err(KindBadJump, "try_end without matching try")
			}
			f.handlers = f.handlers[:len(f.handlers)-1]
			m.shrink(memPerHandler)
		case OpThrow:
			exc, err := m.pop(f)
			if err != nil {
				return Value{}, err
			}
			if rerr := m.unwind(exc); rerr != nil {
				return Value{}, rerr
			}

		case OpHalt:
			var ret Value
			if len(f.stack) > 0 {
				ret = f.stack[len(f.stack)-1]
			}
			return ret, nil
		}
	}
}

// unwind 执行异常栈展开：自内向外查找已注册的异常块，
// 恢复其登记时的栈深度后跳转到处理逻辑。处理逻辑（清理逻辑）按
// 注册逆序（内层优先）确定执行；处理逻辑自身再次抛出异常时，
// 该处理器已弹出，异常继续向外层传播（处理器不捕获自身异常）。
// 无任何处理器时返回携带调用轨迹的未捕获异常错误。
func (m *VM) unwind(exc Value) error {
	// 先捕获完整调用轨迹：若最终无人捕获，帧已被展开，轨迹需提前留存。
	trace := make([]FrameInfo, 0, len(m.frames))
	for i := len(m.frames) - 1; i >= 0; i-- {
		trace = append(trace, FrameInfo{Function: m.frames[i].fn.Name, PC: m.frames[i].pc})
	}
	for len(m.frames) > 0 {
		f := &m.frames[len(m.frames)-1]
		if n := len(f.handlers); n > 0 {
			h := f.handlers[n-1]
			// 弹出该处理器（自身不再捕获后续异常）。
			f.handlers = f.handlers[:n-1]
			m.shrink(memPerHandler)
			// 恢复登记时的栈深度。
			m.shrink(len(f.stack) - h.stackLen)
			f.stack = f.stack[:h.stackLen]
			f.pc = h.catchPC
			// 异常值入栈供处理逻辑使用。
			return m.push(f, exc)
		}
		m.popFrame(f)
	}
	return &RuntimeError{
		Kind:    KindUncaughtException,
		Message: fmt.Sprintf("uncaught exception %s", exc),
		Trace:   trace,
	}
}

func boolInt(b bool) Value {
	if b {
		return Int(1)
	}
	return Int(0)
}
