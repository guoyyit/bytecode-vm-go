package vm

import (
	"fmt"
	"math"

	"github.com/guoyyit/bytecode-vm-go/bytecode"
)

// Kind identifies the kind of a runtime value.
type Kind uint8

func (k Kind) String() string {
	switch k {
	case KInt:
		return "int"
	case KFloat:
		return "float"
	case KString:
		return "string"
	case KBool:
		return "bool"
	case KNull:
		return "null"
	case KException:
		return "exception"
	}
	return "unknown"
}

const (
	KInt Kind = iota
	KFloat
	KString
	KBool
	KNull
	KException
)

// Value is a runtime value on the operand stack or in a local.
type Value struct {
	Kind  Kind
	Int   int64
	Float float64
	Str   string
	Bool  bool
	Exc   *Exception
}

// IntValue creates an int value.
func IntValue(v int64) Value { return Value{Kind: KInt, Int: v} }

// FloatValue creates a float value.
func FloatValue(v float64) Value { return Value{Kind: KFloat, Float: v} }

// StringValue creates a string value.
func StringValue(v string) Value { return Value{Kind: KString, Str: v} }

// BoolValue creates a bool value.
func BoolValue(v bool) Value { return Value{Kind: KBool, Bool: v} }

// NullValue creates a null value.
func NullValue() Value { return Value{Kind: KNull} }

// ExceptionValue creates an exception value.
func ExceptionValue(e *Exception) Value { return Value{Kind: KException, Exc: e} }

// FromConstant converts a bytecode constant to a runtime value.
func FromConstant(c bytecode.Value) Value {
	switch c.Kind {
	case bytecode.VInt:
		return IntValue(c.Int)
	case bytecode.VFloat:
		return FloatValue(c.Float)
	case bytecode.VString:
		return StringValue(c.Str)
	}
	return NullValue()
}

// IsTruthy returns the boolean interpretation of a value.
func (v Value) IsTruthy() bool {
	switch v.Kind {
	case KBool:
		return v.Bool
	case KInt:
		return v.Int != 0
	case KFloat:
		return v.Float != 0 && !math.IsNaN(v.Float)
	case KString:
		return len(v.Str) > 0
	case KNull:
		return false
	case KException:
		return v.Exc != nil
	}
	return false
}

// String returns a human-readable representation.
func (v Value) String() string {
	switch v.Kind {
	case KInt:
		return fmt.Sprintf("%d", v.Int)
	case KFloat:
		return fmt.Sprintf("%g", v.Float)
	case KString:
		return v.Str
	case KBool:
		if v.Bool {
			return "true"
		}
		return "false"
	case KNull:
		return "null"
	case KException:
		if v.Exc != nil {
			return v.Exc.String()
		}
		return "exception(null)"
	}
	return "unknown"
}

// memSize returns the memory footprint of a value in bytes.
func (v Value) memSize() int64 {
	switch v.Kind {
	case KString:
		return 16 + int64(len(v.Str))
	case KException:
		if v.Exc != nil {
			return v.Exc.memSize()
		}
		return 48
	}
	return 16
}
