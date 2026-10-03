package vm

// Op 是单字节操作码。指令编码为定长：1 字节操作码 + 定长操作数（小端序）。
type Op byte

const (
	OpNop Op = iota
	OpConst
	OpLoad
	OpStore
	OpPop
	OpDup

	OpAdd
	OpSub
	OpMul
	OpDiv
	OpMod
	OpNeg

	OpAnd
	OpOr
	OpXor
	OpShl
	OpShr
	OpNot

	OpEq
	OpNe
	OpLt
	OpLe
	OpGt
	OpGe

	OpJmp
	OpJmpIfFalse
	OpJmpIfTrue

	OpCall
	OpRet

	OpTry
	OpTryEnd
	OpThrow

	OpHalt
)

// opInfo 描述一个操作码的元信息。
type opInfo struct {
	name       string
	operandN   int    // 操作数字节数（0/2/4）
	sinceMinor uint16 // 引入该操作码的次版本号（用于向后兼容校验）
}

// opTable 以操作码为索引的元信息表，未注册的位置为 nil（即未知操作码）。
var opTable = [256]*opInfo{}

func reg(op Op, name string, operandN int, sinceMinor uint16) {
	opTable[byte(op)] = &opInfo{name: name, operandN: operandN, sinceMinor: sinceMinor}
}

func init() {
	reg(OpNop, "nop", 0, 0)
	reg(OpConst, "const", 2, 0) // u16 常量池索引
	reg(OpLoad, "load", 2, 0)   // u16 局部变量槽
	reg(OpStore, "store", 2, 0) // u16 局部变量槽
	reg(OpPop, "pop", 0, 0)
	reg(OpDup, "dup", 0, 0)

	reg(OpAdd, "add", 0, 0)
	reg(OpSub, "sub", 0, 0)
	reg(OpMul, "mul", 0, 0)
	reg(OpDiv, "div", 0, 0)
	reg(OpMod, "mod", 0, 0)
	reg(OpNeg, "neg", 0, 0)

	reg(OpAnd, "and", 0, 0)
	reg(OpOr, "or", 0, 0)
	reg(OpXor, "xor", 0, 0)
	// v1.1 新增能力：移位与按位取反。声明 minor<1 的字节码使用它们会在加载期被拒绝。
	reg(OpShl, "shl", 0, 1)
	reg(OpShr, "shr", 0, 1)
	reg(OpNot, "not", 0, 1)

	reg(OpEq, "eq", 0, 0)
	reg(OpNe, "ne", 0, 0)
	reg(OpLt, "lt", 0, 0)
	reg(OpLe, "le", 0, 0)
	reg(OpGt, "gt", 0, 0)
	reg(OpGe, "ge", 0, 0)

	reg(OpJmp, "jmp", 4, 0) // u32 绝对目标地址
	reg(OpJmpIfFalse, "jmp_if_false", 4, 0)
	reg(OpJmpIfTrue, "jmp_if_true", 4, 0)

	reg(OpCall, "call", 2, 0) // u16 函数表索引
	reg(OpRet, "ret", 0, 0)

	reg(OpTry, "try", 4, 0) // u32 异常处理入口地址
	reg(OpTryEnd, "try_end", 0, 0)
	reg(OpThrow, "throw", 0, 0)

	reg(OpHalt, "halt", 0, 0)
}

// validOpcode 报告 code 是否为已注册操作码。
func validOpcode(code byte) bool { return opTable[code] != nil }
