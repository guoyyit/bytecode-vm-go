package main

import (
	"fmt"
	"os"

	"github.com/guoyyit/bytecode-vm-go/bytecode"
)

// This program generates example bytecode artifacts.
func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: gen <output-dir>")
		os.Exit(1)
	}
	outDir := os.Args[1]
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}

	genFactorial(outDir)
	genFibonacci(outDir)
	genExceptionDemo(outDir)
	genPowDemo(outDir)
	fmt.Println("generated example bytecode files in", outDir)
}

// genFactorial: factorial(n) = n * factorial(n-1), base case n <= 1.
func genFactorial(outDir string) {
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
	data, err := w.Bytes()
	if err != nil {
		panic(err)
	}
	writeFile(outDir+"/factorial.bvm", data)
}

// genFibonacci: fib(n) = fib(n-1) + fib(n-2), base cases n <= 1 returns n.
func genFibonacci(outDir string) {
	w := bytecode.NewWriter(1, 0)
	one := w.AddIntConstant(1)
	two := w.AddIntConstant(2)

	cb := bytecode.NewCodeBuilder()
	// if n <= 1 return n
	cb.EmitOpU8(bytecode.OpLoadLocal, 0)
	cb.EmitOpU32(bytecode.OpPushInt, one)
	cb.EmitOp(bytecode.OpLe)
	cb.EmitJump(bytecode.OpJmpIfTrue, "base")
	// fib(n-1)
	cb.EmitOpU8(bytecode.OpLoadLocal, 0)
	cb.EmitOpU32(bytecode.OpPushInt, one)
	cb.EmitOp(bytecode.OpSub)
	cb.EmitCall(0, 1)
	// fib(n-2)
	cb.EmitOpU8(bytecode.OpLoadLocal, 0)
	cb.EmitOpU32(bytecode.OpPushInt, two)
	cb.EmitOp(bytecode.OpSub)
	cb.EmitCall(0, 1)
	// add
	cb.EmitOp(bytecode.OpAdd)
	cb.EmitOp(bytecode.OpRetValue)
	cb.Label("base")
	cb.EmitOpU8(bytecode.OpLoadLocal, 0)
	cb.EmitOp(bytecode.OpRetValue)

	code, _ := cb.Code()
	w.AddFunction("fib", 1, 1, code)
	data, err := w.Bytes()
	if err != nil {
		panic(err)
	}
	writeFile(outDir+"/fibonacci.bvm", data)
}

// genExceptionDemo: demonstrates try/catch/finally.
func genExceptionDemo(outDir string) {
	w := bytecode.NewWriter(1, 0)
	msg := w.AddStringConstant("boom")
	answer := w.AddIntConstant(42)

	cb := bytecode.NewCodeBuilder()
	// try { throw "boom" }
	cb.EmitTry("catch", "finally")
	cb.EmitOpU32(bytecode.OpPushString, msg)
	cb.EmitOp(bytecode.OpThrow)
	cb.EmitOp(bytecode.OpEndTry)
	cb.EmitJump(bytecode.OpJmp, "after")
	// catch (e): return 42
	cb.Label("catch")
	cb.EmitOp(bytecode.OpPop) // pop exception
	cb.EmitOpU32(bytecode.OpPushInt, answer)
	cb.EmitOp(bytecode.OpRetValue)
	// finally: cleanup
	cb.Label("finally")
	cb.EmitOp(bytecode.OpNop)
	cb.EmitOp(bytecode.OpEndFinally)
	cb.Label("after")
	cb.EmitOp(bytecode.OpPushNull)
	cb.EmitOp(bytecode.OpRetValue)

	code, _ := cb.Code()
	w.AddFunction("main", 0, 0, code)
	data, err := w.Bytes()
	if err != nil {
		panic(err)
	}
	writeFile(outDir+"/exception.bvm", data)
}

// genPowDemo: demonstrates the v1.1 POW opcode.
func genPowDemo(outDir string) {
	w := bytecode.NewWriter(1, 1) // v1.1
	two := w.AddIntConstant(2)
	ten := w.AddIntConstant(10)

	cb := bytecode.NewCodeBuilder()
	cb.EmitOpU32(bytecode.OpPushInt, two)
	cb.EmitOpU32(bytecode.OpPushInt, ten)
	cb.EmitOp(bytecode.OpPow)
	cb.EmitOp(bytecode.OpRetValue)

	code, _ := cb.Code()
	w.AddFunction("main", 0, 0, code)
	data, err := w.Bytes()
	if err != nil {
		panic(err)
	}
	writeFile(outDir+"/pow.bvm", data)
}

func writeFile(path string, data []byte) {
	if err := os.WriteFile(path, data, 0o644); err != nil {
		panic(err)
	}
	fmt.Printf("wrote %s (%d bytes)\n", path, len(data))
}
