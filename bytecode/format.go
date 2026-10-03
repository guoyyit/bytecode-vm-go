package bytecode

// Binary layout of a bytecode artifact:
//
//   Offset  Size  Field
//   0       4     Magic = {'B','V','M', FormatVersion}
//   4       2     MajorVersion (uint16, little-endian)
//   6       2     MinorVersion (uint16, little-endian)
//   8       2     Flags (uint16, little-endian)
//   10      4     ConstantPoolCount (uint32)
//   14      ...   ConstantPool entries
//   ...     4     FunctionCount (uint32)
//   ...     ...   Function entries
//
// Constant pool entry:
//   1       Type (uint8): 1=int, 2=float, 3=string
//   ...     Value: int -> int64 (8 bytes), float -> float64 (8 bytes),
//                  string -> uint32 length + bytes
//
// Function entry:
//   2       NameLength (uint16)
//   ...     Name bytes
//   1       NumParams (uint8)
//   1       NumLocals (uint8)
//   4       CodeLength (uint32)
//   ...     Code bytes

const (
	Magic0 = 'B'
	Magic1 = 'V'
	Magic2 = 'M'
)

// FormatVersion is the binary format revision. Bumped when the on-disk
// layout changes in an incompatible way.
const FormatVersion = 1

// Supported bytecode semantic versions.
const (
	VersionMajor1 = 1
	VersionMinor0 = 0
	VersionMinor1 = 1
)

// MaxSupportedMajor is the newest major version this loader understands.
const MaxSupportedMajor = 1

// MinSupportedMinor / MaxSupportedMinor define the supported minor range
// within major version 1.
const (
	MinSupportedMinor = 0
	MaxSupportedMinor = 1
)

// Opcode is a single byte instruction.
type Opcode byte

// Version in which an opcode was introduced. Used to reject bytecode that
// declares an older version but uses a newer opcode.
type opcodeVersion struct {
	major, minor uint16
}

func (o opcodeVersion) lessThan(major, minor uint16) bool {
	return o.major < major || (o.major == major && o.minor < minor)
}

// Opcode definitions.
const (
	OpNop Opcode = 0x00

	// Push constants from the constant pool.
	OpPushInt    Opcode = 0x01 // u32 const index
	OpPushFloat  Opcode = 0x02 // u32 const index
	OpPushString Opcode = 0x03 // u32 const index
	OpPushTrue   Opcode = 0x04
	OpPushFalse  Opcode = 0x05
	OpPushNull   Opcode = 0x06

	// Stack manipulation.
	OpPop Opcode = 0x10
	OpDup Opcode = 0x11

	// Arithmetic.
	OpAdd Opcode = 0x20
	OpSub Opcode = 0x21
	OpMul Opcode = 0x22
	OpDiv Opcode = 0x23
	OpMod Opcode = 0x24
	OpNeg Opcode = 0x25

	// Comparison.
	OpEq Opcode = 0x26
	OpNe Opcode = 0x27
	OpLt Opcode = 0x28
	OpLe Opcode = 0x29
	OpGt Opcode = 0x2A
	OpGe Opcode = 0x2B

	// Bitwise.
	OpAnd Opcode = 0x30
	OpOr  Opcode = 0x31
	OpNot Opcode = 0x32

	// Jumps.
	OpJmp       Opcode = 0x40 // i32 target
	OpJmpIfTrue Opcode = 0x41 // i32 target
	OpJmpIfFalse Opcode = 0x42 // i32 target

	// Locals.
	OpLoadLocal  Opcode = 0x50 // u8 local index
	OpStoreLocal Opcode = 0x51 // u8 local index

	// Calls and returns.
	OpCall     Opcode = 0x60 // u32 func index, u8 argc
	OpRet      Opcode = 0x61
	OpRetValue Opcode = 0x62

	// Exception handling.
	OpThrow     Opcode = 0x70
	OpTry       Opcode = 0x71 // i32 handler, i32 finally
	OpEndTry    Opcode = 0x72
	OpEndFinally Opcode = 0x73

	// v1.1 additions.
	OpPow Opcode = 0x80
)

// opcodeVersions maps each opcode to the bytecode version that introduced it.
var opcodeVersions = map[Opcode]opcodeVersion{
	OpNop:        {1, 0},
	OpPushInt:    {1, 0},
	OpPushFloat:  {1, 0},
	OpPushString: {1, 0},
	OpPushTrue:   {1, 0},
	OpPushFalse:  {1, 0},
	OpPushNull:   {1, 0},
	OpPop:        {1, 0},
	OpDup:        {1, 0},
	OpAdd:        {1, 0},
	OpSub:        {1, 0},
	OpMul:        {1, 0},
	OpDiv:        {1, 0},
	OpMod:        {1, 0},
	OpNeg:        {1, 0},
	OpEq:         {1, 0},
	OpNe:         {1, 0},
	OpLt:         {1, 0},
	OpLe:         {1, 0},
	OpGt:         {1, 0},
	OpGe:         {1, 0},
	OpAnd:        {1, 0},
	OpOr:         {1, 0},
	OpNot:        {1, 0},
	OpJmp:        {1, 0},
	OpJmpIfTrue:  {1, 0},
	OpJmpIfFalse: {1, 0},
	OpLoadLocal:  {1, 0},
	OpStoreLocal: {1, 0},
	OpCall:       {1, 0},
	OpRet:        {1, 0},
	OpRetValue:   {1, 0},
	OpThrow:      {1, 0},
	OpTry:        {1, 0},
	OpEndTry:     {1, 0},
	OpEndFinally: {1, 0},
	OpPow:        {1, 1},
}

// opcodeSizes returns the total instruction size (opcode + operands) for a
// given opcode. Returns 0 for unknown opcodes.
func opcodeSize(op Opcode) int {
	switch op {
	case OpNop, OpPushTrue, OpPushFalse, OpPushNull,
		OpPop, OpDup,
		OpAdd, OpSub, OpMul, OpDiv, OpMod, OpNeg,
		OpEq, OpNe, OpLt, OpLe, OpGt, OpGe,
		OpAnd, OpOr, OpNot,
		OpRet, OpRetValue, OpThrow, OpEndTry, OpEndFinally,
		OpPow:
		return 1
	case OpPushInt, OpPushFloat, OpPushString:
		return 5 // 1 + u32
	case OpJmp, OpJmpIfTrue, OpJmpIfFalse:
		return 5 // 1 + i32
	case OpLoadLocal, OpStoreLocal:
		return 2 // 1 + u8
	case OpCall:
		return 6 // 1 + u32 + u8
	case OpTry:
		return 9 // 1 + i32 + i32
	}
	return 0
}
