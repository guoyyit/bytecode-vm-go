package vm

import (
	"encoding/binary"
	"testing"
)

// 最小合法程序：entry 函数仅 const 0 + ret。
func minimalProgram(t *testing.T, minor uint16) *Program {
	t.Helper()
	a := newAsm()
	a.u16(OpConst, 0).op(OpRet)
	return &Program{
		Major: MajorVersion, Minor: minor,
		Consts: []Constant{{Kind: KindInt, I: 0}},
		Funcs:  []Function{{Name: "main", NumParams: 0, NumLocals: 0, Code: a.done(t)}},
		Entry:  0,
	}
}

func TestLoadRoundTrip(t *testing.T) {
	p := mustLoad(t, minimalProgram(t, 0))
	if p.Major != MajorVersion || p.Minor != 0 {
		t.Fatalf("version %d.%d", p.Major, p.Minor)
	}
	v, _, err := New(p, Config{}).Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if v != Int(0) {
		t.Fatalf("got %v, want 0", v)
	}
}

func TestLoadBadMagic(t *testing.T) {
	data, _ := Encode(minimalProgram(t, 0))
	data[0] = 'X'
	loadErrReason(t, data, ReasonBadMagic)
}

func TestLoadBadChecksum(t *testing.T) {
	data, _ := Encode(minimalProgram(t, 0))
	data[len(data)-1] ^= 0xFF
	loadErrReason(t, data, ReasonBadChecksum)
}

func TestLoadTruncated(t *testing.T) {
	// 截断会破坏校验和；这里手工构造“体被截断但校验和正确”的制品。
	body := header(MajorVersion, 0)
	body = binary.LittleEndian.AppendUint32(body, 1) // 声明 1 个常量
	body = append(body, constInt)
	body = binary.LittleEndian.AppendUint64(body, 7)
	// 缺少函数表、入口等后续结构。
	loadErrReason(t, withCRC(body), ReasonTruncated)

	// 连头部都不完整。
	loadErrReason(t, withCRC([]byte(Magic)), ReasonTruncated)
}

func TestLoadUnsupportedMajor(t *testing.T) {
	p := minimalProgram(t, 0)
	p.Major = MajorVersion + 1
	data, _ := Encode(p)
	loadErrReason(t, data, ReasonUnsupportedVersion)
}

func TestLoadUnsupportedMinor(t *testing.T) {
	p := minimalProgram(t, MinorVersion+1)
	data, _ := Encode(p)
	loadErrReason(t, data, ReasonUnsupportedVersion)
}

func TestLoadUnknownFlags(t *testing.T) {
	data, _ := Encode(minimalProgram(t, 0))
	data[8] = 1 // flags 字段
	// 修正校验和，确保拒绝原因是 flags 而非 checksum。
	body := data[:len(data)-4]
	loadErrReason(t, withCRC(body), ReasonUnsupportedVersion)
}

func TestLoadUnknownOpcode(t *testing.T) {
	p := minimalProgram(t, 0)
	p.Funcs[0].Code = []byte{0xFE}
	data, _ := Encode(p)
	loadErrReason(t, data, ReasonUnknownOpcode)
}

func TestLoadBadConstIndex(t *testing.T) {
	p := minimalProgram(t, 0)
	a := newAsm()
	a.u16(OpConst, 9).op(OpRet) // 常量池只有 1 项
	p.Funcs[0].Code = a.done(t)
	data, _ := Encode(p)
	loadErrReason(t, data, ReasonInvalidConstant)
}

func TestLoadBadLocalSlot(t *testing.T) {
	p := minimalProgram(t, 0)
	a := newAsm()
	a.u16(OpLoad, 3).op(OpRet) // 函数没有局部变量
	p.Funcs[0].Code = a.done(t)
	data, _ := Encode(p)
	loadErrReason(t, data, ReasonInvalidFunction)
}

func TestLoadJumpOutOfRange(t *testing.T) {
	p := minimalProgram(t, 0)
	a := newAsm()
	a.u32(OpJmp, 999)
	p.Funcs[0].Code = a.done(t)
	data, _ := Encode(p)
	loadErrReason(t, data, ReasonInvalidFunction)
}

func TestLoadJumpNotOnBoundary(t *testing.T) {
	p := minimalProgram(t, 0)
	a := newAsm()
	a.jmp(OpJmp, "mid")
	a.u16(OpConst, 0) // 3 字节指令，目标指向其操作数中间
	a.label("mid")
	a.op(OpNop)
	a.op(OpRet)
	// 把跳转目标改为 const 指令的操作数字节（pc=6 起指令，操作数在 7）。
	code := a.done(t)
	binary.LittleEndian.PutUint32(code[1:], 7)
	p.Funcs[0].Code = code
	data, _ := Encode(p)
	loadErrReason(t, data, ReasonInvalidFunction)
}

func TestLoadBadEntry(t *testing.T) {
	p := minimalProgram(t, 0)
	p.Entry = 5
	data, _ := Encode(p)
	loadErrReason(t, data, ReasonInvalidEntry)
}

func TestLoadBadCallTarget(t *testing.T) {
	p := minimalProgram(t, 0)
	a := newAsm()
	a.u16(OpCall, 7).op(OpRet)
	p.Funcs[0].Code = a.done(t)
	data, _ := Encode(p)
	loadErrReason(t, data, ReasonInvalidFunction)
}

// 向后兼容：v1.0 制品只使用 v1.0 操作码，必须正常加载执行。
func TestCompatV10ProgramLoads(t *testing.T) {
	a := newAsm()
	a.u16(OpConst, 0).u16(OpConst, 1).op(OpAdd).op(OpRet)
	p := &Program{
		Major: MajorVersion, Minor: 0,
		Consts: []Constant{{Kind: KindInt, I: 2}, {Kind: KindInt, I: 3}},
		Funcs:  []Function{{Name: "main", Code: a.done(t)}},
	}
	prog := mustLoad(t, p)
	v, _, err := New(prog, Config{}).Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if v != Int(5) {
		t.Fatalf("got %v, want 5", v)
	}
}

// v1.0 制品使用 v1.1 才引入的操作码：加载期拒绝并说明版本原因。
func TestCompatNewOpcodeRejectedForOldMinor(t *testing.T) {
	a := newAsm()
	a.u16(OpConst, 0).u16(OpConst, 1).op(OpShl).op(OpRet)
	p := &Program{
		Major: MajorVersion, Minor: 0,
		Consts: []Constant{{Kind: KindInt, I: 1}, {Kind: KindInt, I: 2}},
		Funcs:  []Function{{Name: "main", Code: a.done(t)}},
	}
	data, _ := Encode(p)
	loadErrReason(t, data, ReasonUnsupportedVersion)
}

// 同一程序声明 minor=1 则正常加载执行，行为一致。
func TestCompatNewOpcodeOKForV11(t *testing.T) {
	a := newAsm()
	a.u16(OpConst, 0).u16(OpConst, 1).op(OpShl).op(OpRet)
	p := &Program{
		Major: MajorVersion, Minor: 1,
		Consts: []Constant{{Kind: KindInt, I: 1}, {Kind: KindInt, I: 4}},
		Funcs:  []Function{{Name: "main", Code: a.done(t)}},
	}
	prog := mustLoad(t, p)
	v, _, err := New(prog, Config{}).Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if v != Int(16) {
		t.Fatalf("got %v, want 16", v)
	}
}
