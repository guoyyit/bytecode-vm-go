package vm

import (
	"strings"
	"testing"

	"github.com/guoyyit/bytecode-vm-go/bytecode"
)

// TestStackOverflow: pushing beyond MaxStackDepth is detected.
func TestStackOverflow(t *testing.T) {
	w := bytecode.NewWriter(1, 0)
	one := w.AddIntConstant(1)
	cb := bytecode.NewCodeBuilder()
	// Push 5 values.
	for i := 0; i < 5; i++ {
		cb.EmitOpU32(bytecode.OpPushInt, one)
	}
	cb.EmitOp(bytecode.OpRetValue)
	code, _ := cb.Code()
	w.AddFunction("main", 0, 0, code)
	data, _ := w.Bytes()
	mod, _ := bytecode.Load(data)

	cfg := DefaultConfig()
	cfg.MaxStackDepth = 3 // only allow 3 stack slots
	_, err := Execute(mod, cfg, "main", nil)
	if err == nil {
		t.Fatal("expected stack overflow error, got nil")
	}
	re, ok := err.(*RuntimeError)
	if !ok {
		t.Fatalf("expected RuntimeError, got %T", err)
	}
	if re.Kind != ErrStackOverflow {
		t.Fatalf("expected ErrStackOverflow, got %v", re.Kind)
	}
}

// TestStackUnderflow: popping from empty stack is detected.
func TestStackUnderflow(t *testing.T) {
	code := []byte{byte(bytecode.OpPop), byte(bytecode.OpRetValue)}
	mod, _ := buildRawModule(1, 0, code)
	_, err := Execute(mod, DefaultConfig(), "main", nil)
	if err == nil {
		t.Fatal("expected stack underflow error, got nil")
	}
	re, ok := err.(*RuntimeError)
	if !ok {
		t.Fatalf("expected RuntimeError, got %T", err)
	}
	if re.Kind != ErrStackUnderflow {
		t.Fatalf("expected ErrStackUnderflow, got %v", re.Kind)
	}
}

// TestInvalidJumpTarget: jumping to a non-instruction boundary is detected.
func TestInvalidJumpTarget(t *testing.T) {
	// Craft code where a jump targets the middle of an instruction.
	// NOP (offset 0), JMP (offset 1) to offset 2 (middle of JMP operands).
	code := []byte{
		byte(bytecode.OpNop),
		byte(bytecode.OpJmp), 0x02, 0x00, 0x00, 0x00, // jump to offset 2
		byte(bytecode.OpRetValue),
	}
	mod, err := buildRawModule(1, 0, code)
	if err != nil {
		t.Fatalf("module should load: %v", err)
	}
	_, err = Execute(mod, DefaultConfig(), "main", nil)
	if err == nil {
		t.Fatal("expected invalid jump target error, got nil")
	}
	re, ok := err.(*RuntimeError)
	if !ok {
		t.Fatalf("expected RuntimeError, got %T", err)
	}
	if re.Kind != ErrInvalidJumpTarget {
		t.Fatalf("expected ErrInvalidJumpTarget, got %v", re.Kind)
	}
}

// TestRecursionDepthLimit: infinite recursion hits the call depth limit.
func TestRecursionDepthLimit(t *testing.T) {
	w := bytecode.NewWriter(1, 0)
	cb := bytecode.NewCodeBuilder()
	// main calls itself forever.
	cb.EmitCall(0, 0)
	cb.EmitOp(bytecode.OpRetValue)
	code, _ := cb.Code()
	w.AddFunction("main", 0, 0, code)
	data, _ := w.Bytes()
	mod, _ := bytecode.Load(data)

	cfg := DefaultConfig()
	cfg.MaxCallDepth = 10
	_, err := Execute(mod, cfg, "main", nil)
	if err == nil {
		t.Fatal("expected call depth limit error, got nil")
	}
	re, ok := err.(*RuntimeError)
	if !ok {
		t.Fatalf("expected RuntimeError, got %T", err)
	}
	if re.Kind != ErrCallDepthLimit {
		t.Fatalf("expected ErrCallDepthLimit, got %v", re.Kind)
	}
}

// TestStepLimit: an infinite loop hits the step limit.
func TestStepLimit(t *testing.T) {
	w := bytecode.NewWriter(1, 0)
	cb := bytecode.NewCodeBuilder()
	cb.Label("loop")
	cb.EmitOp(bytecode.OpNop)
	cb.EmitJump(bytecode.OpJmp, "loop")
	code, _ := cb.Code()
	w.AddFunction("main", 0, 0, code)
	data, _ := w.Bytes()
	mod, _ := bytecode.Load(data)

	cfg := DefaultConfig()
	cfg.MaxSteps = 100
	_, err := Execute(mod, cfg, "main", nil)
	if err == nil {
		t.Fatal("expected step limit error, got nil")
	}
	re, ok := err.(*RuntimeError)
	if !ok {
		t.Fatalf("expected RuntimeError, got %T", err)
	}
	if re.Kind != ErrStepLimit {
		t.Fatalf("expected ErrStepLimit, got %v", re.Kind)
	}
}

// TestMemoryLimit: excessive allocation hits the memory limit.
func TestMemoryLimit(t *testing.T) {
	w := bytecode.NewWriter(1, 0)
	cb := bytecode.NewCodeBuilder()
	// Push many values to exceed memory.
	one := w.AddIntConstant(1)
	for i := 0; i < 1000; i++ {
		cb.EmitOpU32(bytecode.OpPushInt, one)
	}
	cb.EmitOp(bytecode.OpRetValue)
	code, _ := cb.Code()
	w.AddFunction("main", 0, 0, code)
	data, _ := w.Bytes()
	mod, _ := bytecode.Load(data)

	cfg := DefaultConfig()
	cfg.MaxMemoryBytes = 100 // very small
	_, err := Execute(mod, cfg, "main", nil)
	if err == nil {
		t.Fatal("expected memory limit error, got nil")
	}
	re, ok := err.(*RuntimeError)
	if !ok {
		t.Fatalf("expected RuntimeError, got %T", err)
	}
	if re.Kind != ErrMemoryLimit {
		t.Fatalf("expected ErrMemoryLimit, got %v", re.Kind)
	}
}

// TestExceptionCatch: throw and catch an exception.
func TestExceptionCatch(t *testing.T) {
	w := bytecode.NewWriter(1, 0)
	msg := w.AddStringConstant("boom")
	answer := w.AddIntConstant(42)

	cb := bytecode.NewCodeBuilder()
	cb.EmitTry("catch", "")
	cb.EmitOpU32(bytecode.OpPushString, msg)
	cb.EmitOp(bytecode.OpThrow)
	cb.EmitOp(bytecode.OpEndTry)
	cb.EmitJump(bytecode.OpJmp, "after")
	cb.Label("catch")
	cb.EmitOp(bytecode.OpPop) // pop exception
	cb.EmitOpU32(bytecode.OpPushInt, answer)
	cb.EmitOp(bytecode.OpRetValue)
	cb.Label("after")
	cb.EmitOp(bytecode.OpPushNull)
	cb.EmitOp(bytecode.OpRetValue)

	code, _ := cb.Code()
	w.AddFunction("main", 0, 0, code)
	data, _ := w.Bytes()
	mod, _ := bytecode.Load(data)

	result, err := Execute(mod, DefaultConfig(), "main", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Value.Int != 42 {
		t.Fatalf("expected 42, got %v", result.Value)
	}
}

// TestExceptionFinally: finally cleanup runs during unwinding.
func TestExceptionFinally(t *testing.T) {
	w := bytecode.NewWriter(1, 0)
	msg := w.AddStringConstant("boom")
	answer := w.AddIntConstant(99)

	cb := bytecode.NewCodeBuilder()
	cb.EmitTry("catch", "finally")
	cb.EmitOpU32(bytecode.OpPushString, msg)
	cb.EmitOp(bytecode.OpThrow)
	cb.EmitOp(bytecode.OpEndTry)
	cb.EmitJump(bytecode.OpJmp, "after")
	cb.Label("catch")
	cb.EmitOp(bytecode.OpPop)
	cb.EmitOpU32(bytecode.OpPushInt, answer)
	cb.EmitOp(bytecode.OpRetValue)
	cb.Label("finally")
	cb.EmitOp(bytecode.OpNop)
	cb.EmitOp(bytecode.OpEndFinally)
	cb.Label("after")
	cb.EmitOp(bytecode.OpPushNull)
	cb.EmitOp(bytecode.OpRetValue)

	code, _ := cb.Code()
	w.AddFunction("main", 0, 0, code)
	data, _ := w.Bytes()
	mod, _ := bytecode.Load(data)

	result, err := Execute(mod, DefaultConfig(), "main", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Value.Int != 99 {
		t.Fatalf("expected 99, got %v", result.Value)
	}
}

// TestExceptionUncaught: uncaught exception returns an error with a trace.
func TestExceptionUncaught(t *testing.T) {
	w := bytecode.NewWriter(1, 0)
	msg := w.AddStringConstant("fatal")

	cb := bytecode.NewCodeBuilder()
	cb.EmitOpU32(bytecode.OpPushString, msg)
	cb.EmitOp(bytecode.OpThrow)
	code, _ := cb.Code()
	w.AddFunction("main", 0, 0, code)
	data, _ := w.Bytes()
	mod, _ := bytecode.Load(data)

	_, err := Execute(mod, DefaultConfig(), "main", nil)
	if err == nil {
		t.Fatal("expected uncaught exception error, got nil")
	}
	re, ok := err.(*RuntimeError)
	if !ok {
		t.Fatalf("expected RuntimeError, got %T", err)
	}
	if re.Kind != ErrUncaughtException {
		t.Fatalf("expected ErrUncaughtException, got %v", re.Kind)
	}
	if !strings.Contains(re.Message, "fatal") {
		t.Fatalf("expected message to contain 'fatal', got %q", re.Message)
	}
	if len(re.Trace) == 0 {
		t.Fatal("expected non-empty trace")
	}
}

// TestExceptionRethrow: cleanup that throws replaces the original exception.
func TestExceptionRethrow(t *testing.T) {
	w := bytecode.NewWriter(1, 0)
	msg1 := w.AddStringConstant("original")
	msg2 := w.AddStringConstant("cleanup-failure")

	cb := bytecode.NewCodeBuilder()
	// try { throw "original" } finally { throw "cleanup-failure" }
	cb.EmitTry("", "finally")
	cb.EmitOpU32(bytecode.OpPushString, msg1)
	cb.EmitOp(bytecode.OpThrow)
	cb.EmitOp(bytecode.OpEndTry)
	cb.EmitOp(bytecode.OpPushNull)
	cb.EmitOp(bytecode.OpRetValue)
	cb.Label("finally")
	cb.EmitOpU32(bytecode.OpPushString, msg2)
	cb.EmitOp(bytecode.OpThrow)
	cb.EmitOp(bytecode.OpEndFinally)

	code, _ := cb.Code()
	w.AddFunction("main", 0, 0, code)
	data, _ := w.Bytes()
	mod, _ := bytecode.Load(data)

	_, err := Execute(mod, DefaultConfig(), "main", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	re, ok := err.(*RuntimeError)
	if !ok {
		t.Fatalf("expected RuntimeError, got %T", err)
	}
	if re.Kind != ErrUncaughtException {
		t.Fatalf("expected ErrUncaughtException, got %v", re.Kind)
	}
	if !strings.Contains(re.Message, "cleanup-failure") {
		t.Fatalf("expected message to contain 'cleanup-failure', got %q", re.Message)
	}
}

// TestExceptionNestedFinally: multiple finally blocks run in LIFO order.
func TestExceptionNestedFinally(t *testing.T) {
	w := bytecode.NewWriter(1, 0)
	msg := w.AddStringConstant("boom")
	track := w.AddStringConstant("track")

	cb := bytecode.NewCodeBuilder()
	// try { try { throw "boom" } finally { track += "A" } } finally { track += "B" }
	cb.EmitTry("", "outer")
	cb.EmitTry("", "inner")
	cb.EmitOpU32(bytecode.OpPushString, msg)
	cb.EmitOp(bytecode.OpThrow)
	cb.EmitOp(bytecode.OpEndTry)
	cb.EmitOp(bytecode.OpPushNull)
	cb.EmitOp(bytecode.OpRetValue)
	cb.Label("inner")
	cb.EmitOpU32(bytecode.OpPushString, track)
	cb.EmitOp(bytecode.OpPop) // just to have an instruction
	cb.EmitOp(bytecode.OpEndFinally)
	cb.Label("outer")
	cb.EmitOpU32(bytecode.OpPushString, track)
	cb.EmitOp(bytecode.OpPop)
	cb.EmitOp(bytecode.OpEndFinally)

	code, _ := cb.Code()
	w.AddFunction("main", 0, 0, code)
	data, _ := w.Bytes()
	mod, _ := bytecode.Load(data)

	_, err := Execute(mod, DefaultConfig(), "main", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	re, ok := err.(*RuntimeError)
	if !ok {
		t.Fatalf("expected RuntimeError, got %T", err)
	}
	if re.Kind != ErrUncaughtException {
		t.Fatalf("expected ErrUncaughtException, got %v", re.Kind)
	}
	if !strings.Contains(re.Message, "boom") {
		t.Fatalf("expected message to contain 'boom', got %q", re.Message)
	}
}
