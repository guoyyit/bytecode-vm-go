package vm

import (
	"testing"

	"github.com/guoyyit/bytecode-vm-go/bytecode"
)

func buildAddModule() (*bytecode.Module, error) {
	cb := bytecode.NewCodeBuilder()
	cb.EmitOpU8(bytecode.OpLoadLocal, 0)
	cb.EmitOpU8(bytecode.OpLoadLocal, 1)
	cb.EmitOp(bytecode.OpAdd)
	cb.EmitOp(bytecode.OpRetValue)
	code, err := cb.Code()
	if err != nil {
		return nil, err
	}

	w := bytecode.NewWriter(1, 0)
	w.AddFunction("add", 2, 2, code)
	data, err := w.Bytes()
	if err != nil {
		return nil, err
	}
	return bytecode.Load(data)
}

func TestSmokeAdd(t *testing.T) {
	mod, err := buildAddModule()
	if err != nil {
		t.Fatal(err)
	}

	result, err := Execute(mod, DefaultConfig(), "add", []Value{IntValue(3), IntValue(4)})
	if err != nil {
		t.Fatal(err)
	}
	if result.Value.Kind != KInt || result.Value.Int != 7 {
		t.Fatalf("expected 7, got %v", result.Value)
	}
	t.Logf("result: %v, steps: %d", result.Value, result.Steps)
}

func TestSmokeFactorial(t *testing.T) {
	// factorial(n): if n <= 1 return 1; else return n * factorial(n-1)
	w := bytecode.NewWriter(1, 0)
	oneIdx := w.AddIntConstant(1)

	cb := bytecode.NewCodeBuilder()
	// load n
	cb.EmitOpU8(bytecode.OpLoadLocal, 0)
	// push 1
	cb.EmitOpU32(bytecode.OpPushInt, oneIdx)
	// n <= 1
	cb.EmitOp(bytecode.OpLe)
	// if true, jump to base case
	cb.EmitJump(bytecode.OpJmpIfTrue, "base")
	// recursive case: n * factorial(n-1)
	cb.EmitOpU8(bytecode.OpLoadLocal, 0) // n
	cb.EmitOpU8(bytecode.OpLoadLocal, 0) // n again
	cb.EmitOpU32(bytecode.OpPushInt, oneIdx)
	cb.EmitOp(bytecode.OpSub)            // n - 1
	cb.EmitCall(0, 1)                   // call factorial(n-1)
	cb.EmitOp(bytecode.OpMul)
	cb.EmitOp(bytecode.OpRetValue)
	cb.Label("base")
	cb.EmitOpU32(bytecode.OpPushInt, oneIdx)
	cb.EmitOp(bytecode.OpRetValue)

	code, err := cb.Code()
	if err != nil {
		t.Fatal(err)
	}
	w.AddFunction("factorial", 1, 1, code)
	data, err := w.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	mod, err := bytecode.Load(data)
	if err != nil {
		t.Fatal(err)
	}

	result, err := Execute(mod, DefaultConfig(), "factorial", []Value{IntValue(5)})
	if err != nil {
		t.Fatal(err)
	}
	if result.Value.Kind != KInt || result.Value.Int != 120 {
		t.Fatalf("expected 120, got %v", result.Value)
	}
	t.Logf("factorial(5) = %v, steps: %d", result.Value, result.Steps)
}
