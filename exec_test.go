package vm

import (
	"testing"
)

// intConsts 构造整数常量池并返回索引映射辅助。
func intConsts(vals ...int64) []Constant {
	cs := make([]Constant, len(vals))
	for i, v := range vals {
		cs[i] = Constant{Kind: KindInt, I: v}
	}
	return cs
}

func runOK(t *testing.T, prog *Program, cfg Config, args ...Value) (Value, Stats) {
	t.Helper()
	v, st, err := New(prog, cfg).Run(args...)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return v, st
}

func TestArithmetic(t *testing.T) {
	// (2+3)*4 - 10/2 % 3 = 20 - 2 = 18
	a := newAsm()
	a.u16(OpConst, 0).u16(OpConst, 1).op(OpAdd) // 5
	a.u16(OpConst, 2).op(OpMul)                 // 20
	a.u16(OpConst, 3).u16(OpConst, 4).op(OpDiv) // 5
	a.u16(OpConst, 5).op(OpMod)                 // 2
	a.op(OpSub).op(OpRet)                       // 18
	p := &Program{
		Major: MajorVersion, Minor: MinorVersion,
		Consts: intConsts(2, 3, 4, 10, 2, 3),
		Funcs:  []Function{{Name: "main", Code: a.done(t)}},
	}
	v, _ := runOK(t, mustLoad(t, p), Config{})
	if v != Int(18) {
		t.Fatalf("got %v, want 18", v)
	}
}

func TestBitwiseAndNeg(t *testing.T) {
	// ((0xF0 & 0x3C) | 0x01) ^ 0xFF, 再 shl 1, shr 1, not 后取反验证
	a := newAsm()
	a.u16(OpConst, 0).u16(OpConst, 1).op(OpAnd)
	a.u16(OpConst, 2).op(OpOr)
	a.u16(OpConst, 3).op(OpXor) // ((0x30)|1)^0xFF = 0x31^0xFF = 0xCE = 206
	a.u16(OpConst, 4).op(OpShl) // 412
	a.u16(OpConst, 4).op(OpShr) // 206
	a.op(OpNeg)                 // -206
	a.op(OpNeg)                 // 206
	a.op(OpRet)
	p := &Program{
		Major: MajorVersion, Minor: MinorVersion,
		Consts: intConsts(0xF0, 0x3C, 0x01, 0xFF, 1),
		Funcs:  []Function{{Name: "main", Code: a.done(t)}},
	}
	v, _ := runOK(t, mustLoad(t, p), Config{})
	if v != Int(206) {
		t.Fatalf("got %v, want 206", v)
	}
}

func TestCompareAndLoop(t *testing.T) {
	// sum = 0; i = 5; while i != 0 { sum += i; i-- }; return sum  => 15
	a := newAsm()
	a.u16(OpConst, 0).u16(OpStore, 0) // sum = 0
	a.u16(OpConst, 1).u16(OpStore, 1) // i = 5
	a.label("loop")
	a.u16(OpLoad, 1).jmp(OpJmpIfFalse, "end")
	a.u16(OpLoad, 0).u16(OpLoad, 1).op(OpAdd).u16(OpStore, 0)
	a.u16(OpLoad, 1).u16(OpConst, 2).op(OpSub).u16(OpStore, 1)
	a.jmp(OpJmp, "loop")
	a.label("end")
	a.u16(OpLoad, 0).op(OpRet)
	p := &Program{
		Major: MajorVersion, Minor: MinorVersion,
		Consts: intConsts(0, 5, 1),
		Funcs:  []Function{{Name: "main", NumLocals: 2, Code: a.done(t)}},
	}
	v, _ := runOK(t, mustLoad(t, p), Config{})
	if v != Int(15) {
		t.Fatalf("got %v, want 15", v)
	}
}

func TestComparisons(t *testing.T) {
	ops := []struct {
		op   Op
		a, b int64
		want int64
	}{
		{OpEq, 3, 3, 1}, {OpEq, 3, 4, 0}, {OpNe, 3, 4, 1},
		{OpLt, 3, 4, 1}, {OpLe, 4, 4, 1}, {OpGt, 5, 4, 1}, {OpGe, 3, 4, 0},
	}
	for _, tc := range ops {
		a := newAsm()
		a.u16(OpConst, 0).u16(OpConst, 1).op(tc.op).op(OpRet)
		p := &Program{
			Major: MajorVersion, Minor: MinorVersion,
			Consts: intConsts(tc.a, tc.b),
			Funcs:  []Function{{Name: "main", Code: a.done(t)}},
		}
		v, _ := runOK(t, mustLoad(t, p), Config{})
		if v != Int(tc.want) {
			t.Fatalf("%d %v %d: got %v, want %d", tc.a, tc.op, tc.b, v, tc.want)
		}
	}
}

func TestDivByZero(t *testing.T) {
	for _, op := range []Op{OpDiv, OpMod} {
		a := newAsm()
		a.u16(OpConst, 0).u16(OpConst, 1).op(op).op(OpRet)
		p := &Program{
			Major: MajorVersion, Minor: MinorVersion,
			Consts: intConsts(1, 0),
			Funcs:  []Function{{Name: "main", Code: a.done(t)}},
		}
		runErrKind(t, mustLoad(t, p), Config{}, KindDivByZero)
	}
}

func TestTypeError(t *testing.T) {
	a := newAsm()
	a.u16(OpConst, 0).u16(OpConst, 1).op(OpAdd).op(OpRet)
	p := &Program{
		Major: MajorVersion, Minor: MinorVersion,
		Consts: []Constant{{Kind: KindInt, I: 1}, {Kind: KindStr, S: "x"}},
		Funcs:  []Function{{Name: "main", Code: a.done(t)}},
	}
	runErrKind(t, mustLoad(t, p), Config{}, KindTypeError)
}

func TestStackUnderflow(t *testing.T) {
	a := newAsm()
	a.op(OpAdd).op(OpRet)
	p := &Program{
		Major: MajorVersion, Minor: MinorVersion,
		Funcs: []Function{{Name: "main", Code: a.done(t)}},
	}
	runErrKind(t, mustLoad(t, p), Config{}, KindStackUnderflow)
}

func TestStackOverflow(t *testing.T) {
	a := newAsm()
	for i := 0; i < 10; i++ {
		a.u16(OpConst, 0)
	}
	a.op(OpRet)
	p := &Program{
		Major: MajorVersion, Minor: MinorVersion,
		Consts: intConsts(1),
		Funcs:  []Function{{Name: "main", Code: a.done(t)}},
	}
	runErrKind(t, mustLoad(t, p), Config{MaxStack: 4}, KindStackOverflow)
}

// fact(n): if n <= 1 { 1 } else { n * fact(n-1) }
func factProgram(t *testing.T) *Program {
	a := newAsm()
	a.u16(OpLoad, 0).u16(OpConst, 0).op(OpLe).jmp(OpJmpIfFalse, "rec")
	a.u16(OpConst, 0).op(OpRet)
	a.label("rec")
	a.u16(OpLoad, 0)
	a.u16(OpLoad, 0).u16(OpConst, 0).op(OpSub)
	a.u16(OpCall, 0)
	a.op(OpMul).op(OpRet)
	return &Program{
		Major: MajorVersion, Minor: MinorVersion,
		Consts: intConsts(1),
		Funcs:  []Function{{Name: "fact", NumParams: 1, NumLocals: 1, Code: a.done(t)}},
	}
}

func TestRecursion(t *testing.T) {
	prog := mustLoad(t, factProgram(t))
	v, st := runOK(t, prog, Config{}, Int(10))
	if v != Int(3628800) {
		t.Fatalf("got %v, want 3628800", v)
	}
	if st.Calls != 10 {
		t.Fatalf("calls %d, want 10", st.Calls)
	}
}

func TestCallDepthExceeded(t *testing.T) {
	// 无限递归函数 f(): f()
	a := newAsm()
	a.u16(OpCall, 0).op(OpRet)
	p := &Program{
		Major: MajorVersion, Minor: MinorVersion,
		Funcs: []Function{{Name: "f", Code: a.done(t)}},
	}
	prog := mustLoad(t, p)
	re := runErrKind(t, prog, Config{MaxCallDepth: 16}, KindCallDepthExceeded)
	if len(re.Trace) != 16 {
		t.Fatalf("trace depth %d, want 16", len(re.Trace))
	}
}

func TestStepLimit(t *testing.T) {
	a := newAsm()
	a.label("loop").jmp(OpJmp, "loop")
	p := &Program{
		Major: MajorVersion, Minor: MinorVersion,
		Funcs: []Function{{Name: "main", Code: a.done(t)}},
	}
	prog := mustLoad(t, p)
	_, st, err := New(prog, Config{MaxSteps: 100}).Run()
	if err == nil {
		t.Fatal("want step limit error")
	}
	if re := err.(*RuntimeError); re.Kind != KindStepLimitExceeded {
		t.Fatalf("kind %v, want %v", re.Kind, KindStepLimitExceeded)
	}
	if st.Instructions != 100 {
		t.Fatalf("instructions %d, want 100", st.Instructions)
	}
}

func TestMemoryLimit(t *testing.T) {
	// 无限递归，调用深度上限调高，让内存上限先触发。
	a := newAsm()
	a.u16(OpCall, 0).op(OpRet)
	p := &Program{
		Major: MajorVersion, Minor: MinorVersion,
		Funcs: []Function{{Name: "f", NumLocals: 4, Code: a.done(t)}},
	}
	prog := mustLoad(t, p)
	runErrKind(t, prog, Config{MaxCallDepth: 100000, MaxMemory: 100}, KindMemoryLimitExceeded)
}

func TestDeterminism(t *testing.T) {
	prog := mustLoad(t, factProgram(t))
	v1, s1, err1 := New(prog, Config{}).Run(Int(12))
	v2, s2, err2 := New(prog, Config{}).Run(Int(12))
	if err1 != nil || err2 != nil {
		t.Fatalf("errors: %v %v", err1, err2)
	}
	if v1 != v2 || s1 != s2 {
		t.Fatalf("non-deterministic: (%v,%v) vs (%v,%v)", v1, s1, v2, s2)
	}
	// 超限场景同样确定。
	_, _, e1 := New(prog, Config{MaxCallDepth: 5}).Run(Int(12))
	_, _, e2 := New(prog, Config{MaxCallDepth: 5}).Run(Int(12))
	if e1.Error() != e2.Error() {
		t.Fatalf("non-deterministic errors: %v vs %v", e1, e2)
	}
}

func TestHalt(t *testing.T) {
	a := newAsm()
	a.u16(OpConst, 0).op(OpHalt)
	a.u16(OpConst, 1).op(OpRet) // 不可达
	p := &Program{
		Major: MajorVersion, Minor: MinorVersion,
		Consts: intConsts(42, 99),
		Funcs:  []Function{{Name: "main", Code: a.done(t)}},
	}
	v, _ := runOK(t, mustLoad(t, p), Config{})
	if v != Int(42) {
		t.Fatalf("got %v, want 42", v)
	}
}
