package vm

import (
	"testing"

	"github.com/guoyyit/bytecode-vm-go/bytecode"
)

// TestVersionCompatV10Loads: a v1.0 module loads and runs on the v1.1 VM.
func TestVersionCompatV10Loads(t *testing.T) {
	cb := bytecode.NewCodeBuilder()
	cb.EmitOpU8(bytecode.OpLoadLocal, 0)
	cb.EmitOpU32(bytecode.OpPushInt, 0) // placeholder, will fix below
	cb.EmitOp(bytecode.OpAdd)
	cb.EmitOp(bytecode.OpRetValue)
	code, _ := cb.Code()

	w := bytecode.NewWriter(1, 0)
	one := w.AddIntConstant(1)
	// Rebuild code with correct const index.
	cb2 := bytecode.NewCodeBuilder()
	cb2.EmitOpU8(bytecode.OpLoadLocal, 0)
	cb2.EmitOpU32(bytecode.OpPushInt, one)
	cb2.EmitOp(bytecode.OpAdd)
	cb2.EmitOp(bytecode.OpRetValue)
	code, _ = cb2.Code()
	w.AddFunction("inc", 1, 1, code)
	data, err := w.Bytes()
	if err != nil {
		t.Fatal(err)
	}

	mod, err := bytecode.Load(data)
	if err != nil {
		t.Fatalf("v1.0 module should load: %v", err)
	}
	if mod.VersionString() != "1.0" {
		t.Fatalf("expected version 1.0, got %s", mod.VersionString())
	}

	result, err := Execute(mod, DefaultConfig(), "inc", []Value{IntValue(41)})
	if err != nil {
		t.Fatal(err)
	}
	if result.Value.Int != 42 {
		t.Fatalf("expected 42, got %v", result.Value)
	}
}

// TestVersionCompatV11Loads: a v1.1 module using POW loads and runs.
func TestVersionCompatV11Loads(t *testing.T) {
	w := bytecode.NewWriter(1, 1)
	two := w.AddIntConstant(2)
	three := w.AddIntConstant(3)

	cb := bytecode.NewCodeBuilder()
	cb.EmitOpU32(bytecode.OpPushInt, two)
	cb.EmitOpU32(bytecode.OpPushInt, three)
	cb.EmitOp(bytecode.OpPow)
	cb.EmitOp(bytecode.OpRetValue)
	code, _ := cb.Code()
	w.AddFunction("main", 0, 0, code)
	data, err := w.Bytes()
	if err != nil {
		t.Fatal(err)
	}

	mod, err := bytecode.Load(data)
	if err != nil {
		t.Fatalf("v1.1 module should load: %v", err)
	}
	if mod.VersionString() != "1.1" {
		t.Fatalf("expected version 1.1, got %s", mod.VersionString())
	}

	result, err := Execute(mod, DefaultConfig(), "main", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Value.Int != 8 {
		t.Fatalf("expected 8, got %v", result.Value)
	}
}

// TestVersionCompatV10UsingV11OpcodeRejected: a v1.0 module using POW is rejected.
func TestVersionCompatV10UsingV11OpcodeRejected(t *testing.T) {
	w := bytecode.NewWriter(1, 0) // declares v1.0
	two := w.AddIntConstant(2)
	three := w.AddIntConstant(3)

	cb := bytecode.NewCodeBuilder()
	cb.EmitOpU32(bytecode.OpPushInt, two)
	cb.EmitOpU32(bytecode.OpPushInt, three)
	cb.EmitOp(bytecode.OpPow) // v1.1 opcode
	cb.EmitOp(bytecode.OpRetValue)
	code, _ := cb.Code()
	w.AddFunction("main", 0, 0, code)
	data, err := w.Bytes()
	if err != nil {
		t.Fatal(err)
	}

	_, err = bytecode.Load(data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	le, ok := err.(*bytecode.LoadError)
	if !ok {
		t.Fatalf("expected LoadError, got %T", err)
	}
	if le.Kind != bytecode.ErrOpcodeVersionMismatch {
		t.Fatalf("expected ErrOpcodeVersionMismatch, got %v", le.Kind)
	}
}

// TestVersionCompatV12Rejected: a v1.2 module is rejected.
func TestVersionCompatV12Rejected(t *testing.T) {
	w := bytecode.NewWriter(1, 0)
	w.AddFunction("main", 0, 0, []byte{byte(bytecode.OpNop), byte(bytecode.OpRetValue)})
	data, _ := w.Bytes()
	data[6] = 2 // minor version 2
	_, err := bytecode.Load(data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	le, ok := err.(*bytecode.LoadError)
	if !ok {
		t.Fatalf("expected LoadError, got %T", err)
	}
	if le.Kind != bytecode.ErrUnsupportedVersion {
		t.Fatalf("expected ErrUnsupportedVersion, got %v", le.Kind)
	}
}

// TestVersionCompatBehaviorConsistent: v1.0 and v1.1 produce same results for v1.0 ops.
func TestVersionCompatBehaviorConsistent(t *testing.T) {
	buildModule := func(minor uint16) *bytecode.Module {
		w := bytecode.NewWriter(1, minor)
		a := w.AddIntConstant(10)
		b := w.AddIntConstant(3)
		cb := bytecode.NewCodeBuilder()
		cb.EmitOpU32(bytecode.OpPushInt, a)
		cb.EmitOpU32(bytecode.OpPushInt, b)
		cb.EmitOp(bytecode.OpMul)
		cb.EmitOpU32(bytecode.OpPushInt, a)
		cb.EmitOp(bytecode.OpAdd)
		cb.EmitOp(bytecode.OpRetValue)
		code, _ := cb.Code()
		w.AddFunction("main", 0, 0, code)
		data, _ := w.Bytes()
		mod, _ := bytecode.Load(data)
		return mod
	}

	v10 := buildModule(0)
	v11 := buildModule(1)

	r10, err := Execute(v10, DefaultConfig(), "main", nil)
	if err != nil {
		t.Fatal(err)
	}
	r11, err := Execute(v11, DefaultConfig(), "main", nil)
	if err != nil {
		t.Fatal(err)
	}

	if r10.Value.Int != r11.Value.Int {
		t.Fatalf("behavior differs: v1.0=%d, v1.1=%d", r10.Value.Int, r11.Value.Int)
	}
	if r10.Steps != r11.Steps {
		t.Fatalf("step count differs: v1.0=%d, v1.1=%d", r10.Steps, r11.Steps)
	}
	if r10.Value.Int != 40 {
		t.Fatalf("expected 40, got %d", r10.Value.Int)
	}
}
