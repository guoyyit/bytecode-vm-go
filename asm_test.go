package vm

import (
	"encoding/binary"
	"hash/crc32"
	"testing"
)

// asm 是测试用的迷你汇编器，支持前向标签。
type asm struct {
	code   []byte
	labels map[string]int
	fixups []fixup
}

type fixup struct {
	at    int // 操作数（u32）在 code 中的偏移
	label string
}

func newAsm() *asm { return &asm{labels: map[string]int{}} }

func (a *asm) op(o Op) *asm {
	a.code = append(a.code, byte(o))
	return a
}

func (a *asm) u16(o Op, v uint16) *asm {
	a.code = append(a.code, byte(o))
	a.code = binary.LittleEndian.AppendUint16(a.code, v)
	return a
}

func (a *asm) u32(o Op, v uint32) *asm {
	a.code = append(a.code, byte(o))
	a.code = binary.LittleEndian.AppendUint32(a.code, v)
	return a
}

func (a *asm) label(name string) *asm {
	a.labels[name] = len(a.code)
	return a
}

// jmp 发射带标签操作数的跳转/try 指令，done 时回填。
func (a *asm) jmp(o Op, label string) *asm {
	a.code = append(a.code, byte(o))
	a.fixups = append(a.fixups, fixup{at: len(a.code), label: label})
	a.code = append(a.code, 0, 0, 0, 0)
	return a
}

func (a *asm) done(t *testing.T) []byte {
	t.Helper()
	for _, fx := range a.fixups {
		target, ok := a.labels[fx.label]
		if !ok {
			t.Fatalf("undefined label %q", fx.label)
		}
		binary.LittleEndian.PutUint32(a.code[fx.at:], uint32(target))
	}
	return a.code
}

// mustLoad 编码并加载程序，任一环节失败即终止测试。
func mustLoad(t *testing.T, p *Program) *Program {
	t.Helper()
	data, err := Encode(p)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	prog, err := Load(data)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return prog
}

// loadErrReason 加载原始字节并断言失败原因。
func loadErrReason(t *testing.T, data []byte, want LoadReason) {
	t.Helper()
	_, err := Load(data)
	if err == nil {
		t.Fatalf("Load succeeded, want reason %v", want)
	}
	le, ok := err.(*LoadError)
	if !ok {
		t.Fatalf("error type %T, want *LoadError", err)
	}
	if le.Reason != want {
		t.Fatalf("reason %v, want %v (err: %v)", le.Reason, want, le)
	}
}

// withCRC 为手工构造的制品体附加正确校验和。
func withCRC(body []byte) []byte {
	sum := crc32.ChecksumIEEE(body)
	return binary.LittleEndian.AppendUint32(body, sum)
}

// header 构造制品头（magic + 版本 + flags）。
func header(major, minor uint16) []byte {
	b := []byte(Magic)
	b = binary.LittleEndian.AppendUint16(b, major)
	b = binary.LittleEndian.AppendUint16(b, minor)
	b = binary.LittleEndian.AppendUint16(b, 0)
	return b
}

// runErrKind 运行程序并断言运行时错误类别。
func runErrKind(t *testing.T, prog *Program, cfg Config, want RuntimeKind, args ...Value) *RuntimeError {
	t.Helper()
	_, _, err := New(prog, cfg).Run(args...)
	if err == nil {
		t.Fatalf("Run succeeded, want kind %v", want)
	}
	re, ok := err.(*RuntimeError)
	if !ok {
		t.Fatalf("error type %T, want *RuntimeError", err)
	}
	if re.Kind != want {
		t.Fatalf("kind %v, want %v (err: %v)", re.Kind, want, re)
	}
	return re
}
