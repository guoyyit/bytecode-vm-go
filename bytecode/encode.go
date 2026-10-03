package bytecode

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
)

// Writer builds a bytecode artifact.
type Writer struct {
	major, minor uint16
	flags        uint16
	constants    []Value
	functions    []*Function
}

// NewWriter creates a writer for the given bytecode version.
func NewWriter(major, minor uint16) *Writer {
	return &Writer{major: major, minor: minor}
}

// SetFlags sets the artifact flags.
func (w *Writer) SetFlags(flags uint16) { w.flags = flags }

// AddIntConstant adds an int64 constant and returns its index.
func (w *Writer) AddIntConstant(v int64) uint32 {
	w.constants = append(w.constants, Value{Kind: VInt, Int: v})
	return uint32(len(w.constants) - 1)
}

// AddFloatConstant adds a float64 constant and returns its index.
func (w *Writer) AddFloatConstant(v float64) uint32 {
	w.constants = append(w.constants, Value{Kind: VFloat, Float: v})
	return uint32(len(w.constants) - 1)
}

// AddStringConstant adds a string constant and returns its index.
func (w *Writer) AddStringConstant(v string) uint32 {
	w.constants = append(w.constants, Value{Kind: VString, Str: v})
	return uint32(len(w.constants) - 1)
}

// AddFunction adds a function and returns its index.
func (w *Writer) AddFunction(name string, numParams, numLocals uint8, code []byte) uint32 {
	w.functions = append(w.functions, &Function{
		Name:      name,
		NumParams: numParams,
		NumLocals: numLocals,
		Code:      code,
	})
	return uint32(len(w.functions) - 1)
}

// Bytes serializes the artifact.
func (w *Writer) Bytes() ([]byte, error) {
	if len(w.functions) == 0 {
		return nil, newLoadError(ErrEmptyModule, -1, "module contains no functions")
	}

	var buf bytes.Buffer
	// Header.
	buf.WriteByte(Magic0)
	buf.WriteByte(Magic1)
	buf.WriteByte(Magic2)
	buf.WriteByte(FormatVersion)
	writeU16(&buf, w.major)
	writeU16(&buf, w.minor)
	writeU16(&buf, w.flags)

	// Constant pool.
	writeU32(&buf, uint32(len(w.constants)))
	for i, c := range w.constants {
		switch c.Kind {
		case VInt:
			buf.WriteByte(byte(VInt))
			writeI64(&buf, c.Int)
		case VFloat:
			buf.WriteByte(byte(VFloat))
			writeI64(&buf, int64(math.Float64bits(c.Float)))
		case VString:
			buf.WriteByte(byte(VString))
			if len(c.Str) > math.MaxUint32 {
				return nil, newLoadErrorf(ErrInvalidCode, -1, "constant %d string too long", i)
			}
			writeU32(&buf, uint32(len(c.Str)))
			buf.WriteString(c.Str)
		default:
			return nil, newLoadErrorf(ErrInvalidCode, -1, "constant %d has invalid kind %d", i, c.Kind)
		}
	}

	// Function table.
	writeU32(&buf, uint32(len(w.functions)))
	for i, f := range w.functions {
		if len(f.Name) > math.MaxUint16 {
			return nil, newLoadErrorf(ErrInvalidCode, -1, "function %d name too long", i)
		}
		writeU16(&buf, uint16(len(f.Name)))
		buf.WriteString(f.Name)
		buf.WriteByte(f.NumParams)
		buf.WriteByte(f.NumLocals)
		if len(f.Code) > math.MaxUint32 {
			return nil, newLoadErrorf(ErrInvalidCode, -1, "function %d code too long", i)
		}
		writeU32(&buf, uint32(len(f.Code)))
		buf.Write(f.Code)
	}

	return buf.Bytes(), nil
}

func writeU16(buf *bytes.Buffer, v uint16) {
	var b [2]byte
	binary.LittleEndian.PutUint16(b[:], v)
	buf.Write(b[:])
}

func writeU32(buf *bytes.Buffer, v uint32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	buf.Write(b[:])
}

func writeI64(buf *bytes.Buffer, v int64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], uint64(v))
	buf.Write(b[:])
}

// CodeBuilder helps build function code with label fixups.
type CodeBuilder struct {
	code   []byte
	labels map[string]int
	fixups []fixup
}

type fixup struct {
	label string
	pos   int // offset of the i32 operand in code
}

// NewCodeBuilder creates a new code builder.
func NewCodeBuilder() *CodeBuilder {
	return &CodeBuilder{labels: make(map[string]int)}
}

// Emit appends raw bytes.
func (cb *CodeBuilder) Emit(b ...byte) { cb.code = append(cb.code, b...) }

// EmitOp appends an opcode.
func (cb *CodeBuilder) EmitOp(op Opcode) { cb.code = append(cb.code, byte(op)) }

// EmitOpU8 appends an opcode with a u8 operand.
func (cb *CodeBuilder) EmitOpU8(op Opcode, v uint8) {
	cb.code = append(cb.code, byte(op), v)
}

// EmitOpU32 appends an opcode with a u32 operand.
func (cb *CodeBuilder) EmitOpU32(op Opcode, v uint32) {
	cb.code = append(cb.code, byte(op))
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	cb.code = append(cb.code, b[:]...)
}

// EmitOpI32 appends an opcode with an i32 operand.
func (cb *CodeBuilder) EmitOpI32(op Opcode, v int32) {
	cb.code = append(cb.code, byte(op))
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], uint32(v))
	cb.code = append(cb.code, b[:]...)
}

// EmitCall appends a CALL instruction.
func (cb *CodeBuilder) EmitCall(funcIdx uint32, argc uint8) {
	cb.code = append(cb.code, byte(OpCall))
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], funcIdx)
	cb.code = append(cb.code, b[:]...)
	cb.code = append(cb.code, argc)
}

// EmitTry appends a TRY instruction with handler and finally labels.
// Use "" for either to indicate no handler/finally.
func (cb *CodeBuilder) EmitTry(handlerLabel, finallyLabel string) {
	cb.code = append(cb.code, byte(OpTry))
	// placeholder; fixup later
	cb.code = append(cb.code, 0, 0, 0, 0, 0, 0, 0, 0)
	pos := len(cb.code) - 8
	cb.fixups = append(cb.fixups, fixup{label: handlerLabel, pos: pos}, fixup{label: finallyLabel, pos: pos + 4})
}

// Label marks the current position with a name.
func (cb *CodeBuilder) Label(name string) {
	cb.labels[name] = len(cb.code)
}

// EmitJump appends a jump to a label.
func (cb *CodeBuilder) EmitJump(op Opcode, label string) {
	cb.code = append(cb.code, byte(op))
	cb.code = append(cb.code, 0, 0, 0, 0)
	cb.fixups = append(cb.fixups, fixup{label: label, pos: len(cb.code) - 4})
}

// Code returns the built code, resolving label fixups.
func (cb *CodeBuilder) Code() ([]byte, error) {
	for _, f := range cb.fixups {
		var target int32 = -1 // default: no handler/finally
		if f.label != "" {
			t, ok := cb.labels[f.label]
			if !ok {
				return nil, fmt.Errorf("undefined label %q", f.label)
			}
			target = int32(t)
		}
		if f.pos+4 > len(cb.code) {
			return nil, fmt.Errorf("fixup out of range")
		}
		binary.LittleEndian.PutUint32(cb.code[f.pos:f.pos+4], uint32(target))
	}
	return cb.code, nil
}
