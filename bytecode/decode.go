package bytecode

import (
	"encoding/binary"
	"fmt"
	"math"
)

// Reader decodes a bytecode artifact with full structural validation.
type Reader struct {
	data []byte
	pos  int
}

// Load parses and validates a bytecode artifact.
func Load(data []byte) (*Module, error) {
	r := &Reader{data: data}
	return r.parse()
}

func (r *Reader) parse() (*Module, error) {
	if len(r.data) < 10 {
		return nil, newLoadError(ErrTruncated, len(r.data), "artifact too short for header")
	}

	// Magic.
	if r.data[0] != Magic0 || r.data[1] != Magic1 || r.data[2] != Magic2 {
		return nil, newLoadError(ErrBadMagic, 0, "bad magic bytes")
	}
	if r.data[3] != FormatVersion {
		return nil, newLoadErrorf(ErrBadFormatVersion, 3, "unsupported format version %d (expected %d)", r.data[3], FormatVersion)
	}
	r.pos = 4

	major := r.readU16()
	minor := r.readU16()
	flags := r.readU16()

	if major != MaxSupportedMajor {
		return nil, newLoadErrorf(ErrUnsupportedVersion, 4, "unsupported major version %d (supported %d.x)", major, MaxSupportedMajor)
	}
	if minor < MinSupportedMinor || minor > MaxSupportedMinor {
		return nil, newLoadErrorf(ErrUnsupportedVersion, 6, "unsupported minor version %d.%d (supported %d.%d-%d.%d)",
			major, minor, MaxSupportedMajor, MinSupportedMinor, MaxSupportedMajor, MaxSupportedMinor)
	}

	mod := &Module{
		FormatVersion: FormatVersion,
		MajorVersion:  major,
		MinorVersion:  minor,
		Flags:         flags,
	}

	// Constant pool.
	constCount := r.readU32()
	if constCount > uint32(len(r.data)) {
		return nil, newLoadError(ErrTruncated, r.pos, "constant pool count exceeds artifact size")
	}
	mod.Constants = make([]Value, constCount)
	for i := uint32(0); i < constCount; i++ {
		c, err := r.readConstant()
		if err != nil {
			return nil, err
		}
		mod.Constants[i] = c
	}

	// Function table.
	funcCount := r.readU32()
	if funcCount == 0 {
		return nil, newLoadError(ErrEmptyModule, r.pos, "module contains no functions")
	}
	if funcCount > uint32(len(r.data)) {
		return nil, newLoadError(ErrTruncated, r.pos, "function count exceeds artifact size")
	}
	mod.Functions = make([]*Function, funcCount)
	for i := uint32(0); i < funcCount; i++ {
		f, err := r.readFunctionHeader()
		if err != nil {
			return nil, err
		}
		mod.Functions[i] = f
	}
	// Validate all function code after the full table is available.
	for i := uint32(0); i < funcCount; i++ {
		if err := r.validateCode(mod.Functions[i], mod); err != nil {
			return nil, err
		}
	}

	if r.pos != len(r.data) {
		return nil, newLoadErrorf(ErrInvalidCode, r.pos, "trailing %d bytes after function table", len(r.data)-r.pos)
	}

	return mod, nil
}

func (r *Reader) readConstant() (Value, error) {
	start := r.pos
	kind := r.readU8()
	switch ValueKind(kind) {
	case VInt:
		v := r.readI64()
		return Value{Kind: VInt, Int: v}, nil
	case VFloat:
		bits := r.readU64()
		return Value{Kind: VFloat, Float: math.Float64frombits(bits)}, nil
	case VString:
		n := r.readU32()
		if uint64(n) > uint64(len(r.data)-r.pos) {
			return Value{}, newLoadError(ErrTruncated, start, "string constant length exceeds artifact size")
		}
		s := string(r.data[r.pos : r.pos+int(n)])
		r.pos += int(n)
		return Value{Kind: VString, Str: s}, nil
	default:
		return Value{}, newLoadErrorf(ErrBadConstantType, start, "unknown constant type %d", kind)
	}
}

func (r *Reader) readFunctionHeader() (*Function, error) {
	start := r.pos
	nameLen := r.readU16()
	if uint64(nameLen) > uint64(len(r.data)-r.pos) {
		return nil, newLoadError(ErrTruncated, start, "function name length exceeds artifact size")
	}
	name := string(r.data[r.pos : r.pos+int(nameLen)])
	r.pos += int(nameLen)

	numParams := r.readU8()
	numLocals := r.readU8()
	if numLocals < numParams {
		return nil, newLoadErrorf(ErrInvalidCode, start, "function %q: numLocals (%d) < numParams (%d)", name, numLocals, numParams)
	}

	codeLen := r.readU32()
	if uint64(codeLen) > uint64(len(r.data)-r.pos) {
		return nil, newLoadError(ErrTruncated, start, "function code length exceeds artifact size")
	}
	code := make([]byte, codeLen)
	copy(code, r.data[r.pos:r.pos+int(codeLen)])
	r.pos += int(codeLen)

	return &Function{
		Name:      name,
		NumParams: numParams,
		NumLocals: numLocals,
		Code:      code,
	}, nil
}

// validateCode parses every instruction, checks opcode versions, operand
// ranges, and builds the valid-instruction-start bitmap.
func (r *Reader) validateCode(f *Function, mod *Module) error {
	f.validInstStarts = make([]bool, len(f.Code))
	pos := 0
	for pos < len(f.Code) {
		f.validInstStarts[pos] = true
		op := Opcode(f.Code[pos])
		ver, known := opcodeVersions[op]
		if !known {
			return newLoadErrorf(ErrBadOpcode, pos, "function %q: unknown opcode 0x%02x", f.Name, op)
		}
		// The module's declared version must be >= the opcode's minimum version.
		if mod.MajorVersion < ver.major || (mod.MajorVersion == ver.major && mod.MinorVersion < ver.minor) {
			return newLoadErrorf(ErrOpcodeVersionMismatch, pos, "function %q: opcode 0x%02x requires version %d.%d, module declares %d.%d",
				f.Name, op, ver.major, ver.minor, mod.MajorVersion, mod.MinorVersion)
		}

		size := opcodeSize(op)
		if size == 0 {
			return newLoadErrorf(ErrBadOpcode, pos, "function %q: opcode 0x%02x has no size", f.Name, op)
		}
		if pos+size > len(f.Code) {
			return newLoadErrorf(ErrTruncated, pos, "function %q: truncated instruction at offset %d", f.Name, pos)
		}

		// Validate operands.
		switch op {
		case OpPushInt, OpPushFloat, OpPushString:
			idx := binary.LittleEndian.Uint32(f.Code[pos+1 : pos+5])
			if int(idx) >= len(mod.Constants) {
				return newLoadErrorf(ErrConstantIndexOutOfRange, pos, "function %q: constant index %d out of range (pool size %d)", f.Name, idx, len(mod.Constants))
			}
		case OpLoadLocal, OpStoreLocal:
			idx := f.Code[pos+1]
			if idx >= f.NumLocals {
				return newLoadErrorf(ErrLocalIndexOutOfRange, pos, "function %q: local index %d out of range (numLocals %d)", f.Name, idx, f.NumLocals)
			}
		case OpCall:
			funcIdx := binary.LittleEndian.Uint32(f.Code[pos+1 : pos+5])
			argc := f.Code[pos+5]
			if int(funcIdx) >= len(mod.Functions) {
				return newLoadErrorf(ErrFunctionIndexOutOfRange, pos, "function %q: call target index %d out of range (function count %d)", f.Name, funcIdx, len(mod.Functions))
			}
			if int(argc) > int(mod.Functions[funcIdx].NumParams) {
				return newLoadErrorf(ErrArgCountMismatch, pos, "function %q: call to %q with %d args but function takes %d",
					f.Name, mod.Functions[funcIdx].Name, argc, mod.Functions[funcIdx].NumParams)
			}
			// Jump targets are validated at execution time (see vm.validateJumpTarget).
			}

			pos += size
		}
		return nil
	}

func (r *Reader) readU8() uint8 {
	if r.pos >= len(r.data) {
		return 0
	}
	v := r.data[r.pos]
	r.pos++
	return v
}

func (r *Reader) readU16() uint16 {
	if r.pos+2 > len(r.data) {
		return 0
	}
	v := binary.LittleEndian.Uint16(r.data[r.pos : r.pos+2])
	r.pos += 2
	return v
}

func (r *Reader) readU32() uint32 {
	if r.pos+4 > len(r.data) {
		return 0
	}
	v := binary.LittleEndian.Uint32(r.data[r.pos : r.pos+4])
	r.pos += 4
	return v
}

func (r *Reader) readU64() uint64 {
	if r.pos+8 > len(r.data) {
		return 0
	}
	v := binary.LittleEndian.Uint64(r.data[r.pos : r.pos+8])
	r.pos += 8
	return v
}

func (r *Reader) readI64() int64 {
	return int64(r.readU64())
}

// Ensure fmt is used (for potential future use) - keep import valid.
var _ = fmt.Sprintf
