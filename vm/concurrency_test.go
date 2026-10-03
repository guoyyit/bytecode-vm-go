package vm

import (
	"sync"
	"testing"

	"github.com/guoyyit/bytecode-vm-go/bytecode"
)

// buildFactorialModule builds a factorial module for concurrency testing.
func buildFactorialModule() *bytecode.Module {
	w := bytecode.NewWriter(1, 0)
	one := w.AddIntConstant(1)

	cb := bytecode.NewCodeBuilder()
	cb.EmitOpU8(bytecode.OpLoadLocal, 0)
	cb.EmitOpU32(bytecode.OpPushInt, one)
	cb.EmitOp(bytecode.OpLe)
	cb.EmitJump(bytecode.OpJmpIfTrue, "base")
	cb.EmitOpU8(bytecode.OpLoadLocal, 0)
	cb.EmitOpU8(bytecode.OpLoadLocal, 0)
	cb.EmitOpU32(bytecode.OpPushInt, one)
	cb.EmitOp(bytecode.OpSub)
	cb.EmitCall(0, 1)
	cb.EmitOp(bytecode.OpMul)
	cb.EmitOp(bytecode.OpRetValue)
	cb.Label("base")
	cb.EmitOpU32(bytecode.OpPushInt, one)
	cb.EmitOp(bytecode.OpRetValue)

	code, _ := cb.Code()
	w.AddFunction("factorial", 1, 1, code)
	data, _ := w.Bytes()
	mod, _ := bytecode.Load(data)
	return mod
}

// TestConcurrency: multiple goroutines execute the same module concurrently.
func TestConcurrency(t *testing.T) {
	mod := buildFactorialModule()
	cfg := DefaultConfig()

	expected := map[int64]int64{
		0: 1, 1: 1, 2: 2, 3: 6, 4: 24, 5: 120, 6: 720, 7: 5040,
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 64)

	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func(n int64) {
			defer wg.Done()
			result, err := Execute(mod, cfg, "factorial", []Value{IntValue(n)})
			if err != nil {
				errCh <- err
				return
			}
			if result.Value.Int != expected[n] {
				errCh <- err
				return
			}
		}(int64(i % 8))
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent execution error: %v", err)
		}
	}
}

// TestConcurrencyDeterminism: repeated executions with the same config and
// input produce identical results and step counts.
func TestConcurrencyDeterminism(t *testing.T) {
	mod := buildFactorialModule()
	cfg := DefaultConfig()

	const n = 10
	results := make([]*Result, n)
	for i := 0; i < n; i++ {
		result, err := Execute(mod, cfg, "factorial", []Value{IntValue(5)})
		if err != nil {
			t.Fatal(err)
		}
		results[i] = result
	}

	first := results[0]
	for i, r := range results[1:] {
		if r.Value.Int != first.Value.Int {
			t.Fatalf("result differs at index %d: %d vs %d", i+1, r.Value.Int, first.Value.Int)
		}
		if r.Steps != first.Steps {
			t.Fatalf("step count differs at index %d: %d vs %d", i+1, r.Steps, first.Steps)
		}
	}
}

// TestConcurrencyIsolation: an error in one instance does not affect others.
func TestConcurrencyIsolation(t *testing.T) {
	mod := buildFactorialModule()
	cfg := DefaultConfig()

	var wg sync.WaitGroup
	success := make(chan bool, 32)

	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(n int64) {
			defer wg.Done()
			// Some calls will exceed the step limit, others succeed.
			limitedCfg := cfg
			if n%2 == 0 {
				limitedCfg.MaxSteps = 10 // will fail for factorial(5)
			}
			result, err := Execute(mod, limitedCfg, "factorial", []Value{IntValue(5)})
			if n%2 == 0 {
				if err == nil {
					success <- false
					return
				}
				success <- true
			} else {
				if err != nil {
					success <- false
					return
				}
				if result.Value.Int != 120 {
					success <- false
					return
				}
				success <- true
			}
		}(int64(i))
	}

	wg.Wait()
	close(success)

	for ok := range success {
		if !ok {
			t.Fatal("isolation test failed")
		}
	}
}
