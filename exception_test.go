package vm

import (
	"strings"
	"sync"
	"testing"
)

// try H; throw "boom"; H: pop; const 7; ret —— 捕获后返回 7。
func TestExceptionCaught(t *testing.T) {
	a := newAsm()
	a.jmp(OpTry, "handler")
	a.u16(OpConst, 0).op(OpThrow)
	a.label("handler")
	a.op(OpPop).u16(OpConst, 1).op(OpRet)
	p := &Program{
		Major: MajorVersion, Minor: MinorVersion,
		Consts: []Constant{{Kind: KindStr, S: "boom"}, {Kind: KindInt, I: 7}},
		Funcs:  []Function{{Name: "main", Code: a.done(t)}},
	}
	v, _ := runOK(t, mustLoad(t, p), Config{})
	if v != Int(7) {
		t.Fatalf("got %v, want 7", v)
	}
}

// 未捕获异常：main 调用 f，f 抛出，错误携带自内向外的调用轨迹。
func TestUncaughtExceptionTrace(t *testing.T) {
	f := newAsm()
	f.u16(OpConst, 0).op(OpThrow)
	main := newAsm()
	main.u16(OpCall, 1).op(OpRet)
	p := &Program{
		Major: MajorVersion, Minor: MinorVersion,
		Consts: []Constant{{Kind: KindStr, S: "boom"}},
		Funcs: []Function{
			{Name: "main", Code: main.done(t)},
			{Name: "f", Code: f.done(t)},
		},
	}
	prog := mustLoad(t, p)
	re := runErrKind(t, prog, Config{}, KindUncaughtException)
	if len(re.Trace) != 2 || re.Trace[0].Function != "f" || re.Trace[1].Function != "main" {
		t.Fatalf("trace %+v, want [f main]", re.Trace)
	}
	if !strings.Contains(re.Error(), "boom") {
		t.Fatalf("error %q should mention exception value", re.Error())
	}
}

// 嵌套异常块：内层处理逻辑先执行；处理逻辑自身再次抛出时，
// 该处理器已弹出，异常传播到外层处理器（处理器不捕获自身异常）。
func TestNestedHandlersRethrow(t *testing.T) {
	a := newAsm()
	a.jmp(OpTry, "outer")
	a.jmp(OpTry, "inner")
	a.u16(OpConst, 0).op(OpThrow) // throw "x"
	a.label("inner")              // 内层清理：再次抛出
	a.op(OpPop).u16(OpConst, 1).op(OpThrow)
	a.label("outer") // 外层捕获内层重抛的异常
	a.op(OpPop).u16(OpConst, 2).op(OpRet)
	p := &Program{
		Major: MajorVersion, Minor: MinorVersion,
		Consts: []Constant{
			{Kind: KindStr, S: "x"}, {Kind: KindStr, S: "y"}, {Kind: KindInt, I: 9},
		},
		Funcs: []Function{{Name: "main", Code: a.done(t)}},
	}
	v, _ := runOK(t, mustLoad(t, p), Config{})
	if v != Int(9) {
		t.Fatalf("got %v, want 9", v)
	}
}

// try_end 注销处理器：正常离开保护块后抛出的异常不再被该处理器捕获。
func TestTryEndUnregisters(t *testing.T) {
	a := newAsm()
	a.jmp(OpTry, "handler")
	a.op(OpTryEnd)
	a.u16(OpConst, 0).op(OpThrow) // 已无处理器
	a.label("handler")
	a.op(OpPop).u16(OpConst, 1).op(OpRet)
	p := &Program{
		Major: MajorVersion, Minor: MinorVersion,
		Consts: []Constant{{Kind: KindStr, S: "late"}, {Kind: KindInt, I: 7}},
		Funcs:  []Function{{Name: "main", Code: a.done(t)}},
	}
	runErrKind(t, mustLoad(t, p), Config{}, KindUncaughtException)
}

// 跨函数展开：f 抛出，main 中的处理器捕获；f 的帧被展开。
func TestUnwindAcrossFrames(t *testing.T) {
	f := newAsm()
	f.u16(OpConst, 0).op(OpThrow)
	main := newAsm()
	main.jmp(OpTry, "handler")
	main.u16(OpCall, 1).op(OpRet)
	main.label("handler")
	main.op(OpPop).u16(OpConst, 1).op(OpRet)
	p := &Program{
		Major: MajorVersion, Minor: MinorVersion,
		Consts: []Constant{{Kind: KindStr, S: "deep"}, {Kind: KindInt, I: 3}},
		Funcs: []Function{
			{Name: "main", Code: main.done(t)},
			{Name: "f", Code: f.done(t)},
		},
	}
	v, _ := runOK(t, mustLoad(t, p), Config{})
	if v != Int(3) {
		t.Fatalf("got %v, want 3", v)
	}
}

// 展开时操作数栈恢复到 try 登记时的深度。
func TestUnwindRestoresStackDepth(t *testing.T) {
	a := newAsm()
	a.u16(OpConst, 0) // 登记时栈深 1
	a.jmp(OpTry, "handler")
	a.u16(OpConst, 1).u16(OpConst, 1).u16(OpConst, 1) // 栈深涨到 4
	a.u16(OpConst, 2).op(OpThrow)
	a.label("handler") // 展开后栈 = [10, exc]
	a.op(OpPop)        // 丢弃异常值
	a.op(OpRet)        // 返回登记前压入的 10
	p := &Program{
		Major: MajorVersion, Minor: MinorVersion,
		Consts: intConsts(10, 99, 0),
		Funcs:  []Function{{Name: "main", Code: a.done(t)}},
	}
	v, _ := runOK(t, mustLoad(t, p), Config{})
	if v != Int(10) {
		t.Fatalf("got %v, want 10", v)
	}
}

// 并发：同一 Program 被多个实例并发执行，结果一致且互不干扰。
func TestConcurrentInstances(t *testing.T) {
	prog := mustLoad(t, factProgram(t))
	const n = 32
	var wg sync.WaitGroup
	results := make([]Value, n)
	stats := make([]Stats, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			v, st, err := New(prog, Config{}).Run(Int(10))
			if err != nil {
				t.Errorf("instance %d: %v", i, err)
				return
			}
			results[i], stats[i] = v, st
		}(i)
	}
	wg.Wait()
	for i := 0; i < n; i++ {
		if results[i] != Int(3628800) {
			t.Fatalf("instance %d got %v", i, results[i])
		}
		if stats[i] != stats[0] {
			t.Fatalf("instance %d stats %+v != %+v", i, stats[i], stats[0])
		}
	}
}

// 并发下部分实例超限失败、部分成功，互不影响。
func TestConcurrentMixedLimits(t *testing.T) {
	prog := mustLoad(t, factProgram(t))
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cfg := Config{}
			if i%2 == 0 {
				cfg.MaxCallDepth = 3 // 必然超限
			}
			_, _, err := New(prog, cfg).Run(Int(10))
			if i%2 == 0 && err == nil {
				t.Errorf("instance %d: want depth error", i)
			}
			if i%2 == 1 && err != nil {
				t.Errorf("instance %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()
}

// 入口函数参数个数不匹配时以可识别错误拒绝。
func TestEntryArgMismatch(t *testing.T) {
	prog := mustLoad(t, factProgram(t))
	runErrKind(t, prog, Config{}, KindBadCall)                 // 缺参
	runErrKind(t, prog, Config{}, KindBadCall, Int(1), Int(2)) // 多参
}
