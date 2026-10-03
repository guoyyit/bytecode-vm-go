package vm

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

// 版本与格式常量。
const (
	Magic = "BCVM"
	// MajorVersion 是当前支持的主版本，主版本不同一律拒绝。
	MajorVersion = 1
	// MinorVersion 是当前支持的次版本，次版本更低或相等的制品按兼容规则加载。
	MinorVersion = 1
)

// 常量类型标签。
const (
	constInt = 0
	constStr = 1
)

// 制品二进制布局（全部小端序）：
//   magic[4] | major u16 | minor u16 | flags u16(保留,须为0)
//   constCount u32 | consts...
//     const: tag u8 (0=int: i64 | 1=str: u32 len + bytes)
//   funcCount u32 | funcs...
//     func: nameLen u16 + name | numParams u16 | numLocals u16 | codeLen u32 | code
//   entry u32
//   crc32 u32 (对以上全部字节的 IEEE 校验和)

// Constant 是常量池中的一项。
type Constant struct {
	Kind ValueKind
	I    int64
	S    string
}

// Function 是函数表中的一项（加载后只读）。
type Function struct {
	Name      string
	NumParams int
	NumLocals int
	Code      []byte
}

// Program 是加载并校验通过的字节码制品，加载后不可变，可被多个执行实例并发共享。
type Program struct {
	Major, Minor uint16
	Consts       []Constant
	Funcs        []Function
	Entry        int
}

// reader 是带边界检查的字节游标。
type reader struct {
	data []byte
	off  int
}

func (r *reader) remaining() int { return len(r.data) - r.off }

func (r *reader) u8() (byte, bool) {
	if r.remaining() < 1 {
		return 0, false
	}
	v := r.data[r.off]
	r.off++
	return v, true
}

func (r *reader) u16() (uint16, bool) {
	if r.remaining() < 2 {
		return 0, false
	}
	v := binary.LittleEndian.Uint16(r.data[r.off:])
	r.off += 2
	return v, true
}

func (r *reader) u32() (uint32, bool) {
	if r.remaining() < 4 {
		return 0, false
	}
	v := binary.LittleEndian.Uint32(r.data[r.off:])
	r.off += 4
	return v, true
}

func (r *reader) u64() (uint64, bool) {
	if r.remaining() < 8 {
		return 0, false
	}
	v := binary.LittleEndian.Uint64(r.data[r.off:])
	r.off += 8
	return v, true
}

func (r *reader) bytes(n int) ([]byte, bool) {
	if n < 0 || r.remaining() < n {
		return nil, false
	}
	v := r.data[r.off : r.off+n]
	r.off += n
	return v, true
}

// Load 解析并校验字节码。结构损坏、版本不受支持、未知操作码均以
// 可区分原因的 *LoadError 拒绝，绝不返回部分可用的 Program。
func Load(data []byte) (*Program, error) {
	if len(data) < 4+2+2+2+4 { // 头部最小长度
		return nil, loadErr(ReasonTruncated, "artifact too small: %d bytes", len(data))
	}
	if string(data[:4]) != Magic {
		return nil, loadErr(ReasonBadMagic, "magic is %q, want %q", data[:4], Magic)
	}
	// 校验和：末尾 4 字节是对之前全部内容的 CRC32。
	if len(data) < 4 {
		return nil, loadErr(ReasonTruncated, "missing checksum")
	}
	body, sumBytes := data[:len(data)-4], data[len(data)-4:]
	want := binary.LittleEndian.Uint32(sumBytes)
	if got := crc32.ChecksumIEEE(body); got != want {
		return nil, loadErr(ReasonBadChecksum, "crc32 got %#08x, want %#08x", got, want)
	}

	r := &reader{data: body, off: 4}
	major, _ := r.u16()
	minor, _ := r.u16()
	flags, _ := r.u16()
	if major != MajorVersion {
		return nil, loadErr(ReasonUnsupportedVersion,
			"major version %d not supported (this runtime supports major=%d)", major, MajorVersion)
	}
	if minor > MinorVersion {
		return nil, loadErr(ReasonUnsupportedVersion,
			"minor version %d not supported (this runtime supports up to %d.%d)", minor, MajorVersion, MinorVersion)
	}
	if flags != 0 {
		return nil, loadErr(ReasonUnsupportedVersion, "unknown format flags %#04x", flags)
	}
	p := &Program{Major: major, Minor: minor}

	// 常量池
	nConst, ok := r.u32()
	if !ok {
		return nil, loadErr(ReasonTruncated, "reading constant count")
	}
	if nConst > 1<<20 {
		return nil, loadErr(ReasonInvalidConstant, "constant count %d unreasonable", nConst)
	}
	p.Consts = make([]Constant, 0, nConst)
	for i := 0; i < int(nConst); i++ {
		tag, ok := r.u8()
		if !ok {
			return nil, loadErr(ReasonTruncated, "reading constant %d", i)
		}
		switch tag {
		case constInt:
			v, ok := r.u64()
			if !ok {
				return nil, loadErr(ReasonTruncated, "reading int constant %d", i)
			}
			p.Consts = append(p.Consts, Constant{Kind: KindInt, I: int64(v)})
		case constStr:
			n, ok := r.u32()
			if !ok {
				return nil, loadErr(ReasonTruncated, "reading string constant %d length", i)
			}
			b, ok := r.bytes(int(n))
			if !ok {
				return nil, loadErr(ReasonTruncated, "reading string constant %d body", i)
			}
			p.Consts = append(p.Consts, Constant{Kind: KindStr, S: string(b)})
		default:
			return nil, loadErr(ReasonInvalidConstant, "constant %d has unknown tag %d", i, tag)
		}
	}

	// 函数表
	nFunc, ok := r.u32()
	if !ok {
		return nil, loadErr(ReasonTruncated, "reading function count")
	}
	if nFunc == 0 {
		return nil, loadErr(ReasonInvalidFunction, "function table is empty")
	}
	if nFunc > 1<<16 {
		return nil, loadErr(ReasonInvalidFunction, "function count %d unreasonable", nFunc)
	}
	p.Funcs = make([]Function, 0, nFunc)
	for i := 0; i < int(nFunc); i++ {
		nameLen, ok := r.u16()
		if !ok {
			return nil, loadErr(ReasonTruncated, "reading function %d name length", i)
		}
		nb, ok := r.bytes(int(nameLen))
		if !ok {
			return nil, loadErr(ReasonTruncated, "reading function %d name", i)
		}
		numParams, ok1 := r.u16()
		numLocals, ok2 := r.u16()
		codeLen, ok3 := r.u32()
		if !ok1 || !ok2 || !ok3 {
			return nil, loadErr(ReasonTruncated, "reading function %d header", i)
		}
		if int(numParams) > int(numLocals) {
			return nil, loadErr(ReasonInvalidFunction,
				"function %q: numParams %d > numLocals %d", nb, numParams, numLocals)
		}
		code, ok := r.bytes(int(codeLen))
		if !ok {
			return nil, loadErr(ReasonTruncated, "reading function %q code", nb)
		}
		fn := Function{
			Name:      string(nb),
			NumParams: int(numParams),
			NumLocals: int(numLocals),
			Code:      append([]byte(nil), code...), // 拷贝，与输入切片解耦
		}
		if err := validateCode(&fn, p, minor, int(nFunc)); err != nil {
			return nil, err
		}
		p.Funcs = append(p.Funcs, fn)
	}

	// 入口函数
	entry, ok := r.u32()
	if !ok {
		return nil, loadErr(ReasonTruncated, "reading entry index")
	}
	if int(entry) >= len(p.Funcs) {
		return nil, loadErr(ReasonInvalidEntry, "entry index %d out of %d functions", entry, len(p.Funcs))
	}
	p.Entry = int(entry)

	if r.off != len(body) {
		return nil, loadErr(ReasonInvalidFunction, "%d trailing bytes before checksum", len(body)-r.off)
	}
	return p, nil
}

// validateCode 扫描单个函数的指令流，校验操作码、版本门槛、操作数边界、
// 常量/函数索引与跳转目标（必须落在指令边界上且在代码范围内）。
func validateCode(fn *Function, p *Program, minor uint16, nFunc int) error {
	code := fn.Code
	boundary := make([]bool, len(code)+1) // 指令边界标记
	type jumpRef struct{ at, target int }
	var jumps []jumpRef

	pc := 0
	for pc < len(code) {
		boundary[pc] = true
		opByte := code[pc]
		info := opTable[opByte]
		if info == nil {
			return loadErr(ReasonUnknownOpcode,
				"function %q: unknown opcode %#02x at pc=%d", fn.Name, opByte, pc)
		}
		if info.sinceMinor > minor {
			return loadErr(ReasonUnsupportedVersion,
				"function %q: opcode %s requires format >= %d.%d, artifact is %d.%d",
				fn.Name, info.name, MajorVersion, info.sinceMinor, MajorVersion, minor)
		}
		if pc+1+info.operandN > len(code) {
			return loadErr(ReasonInvalidFunction,
				"function %q: truncated operands for %s at pc=%d", fn.Name, info.name, pc)
		}
		switch Op(opByte) {
		case OpConst:
			idx := int(binary.LittleEndian.Uint16(code[pc+1:]))
			if idx >= len(p.Consts) {
				return loadErr(ReasonInvalidConstant,
					"function %q: const index %d out of pool size %d at pc=%d",
					fn.Name, idx, len(p.Consts), pc)
			}
		case OpLoad, OpStore:
			slot := int(binary.LittleEndian.Uint16(code[pc+1:]))
			if slot >= fn.NumLocals {
				return loadErr(ReasonInvalidFunction,
					"function %q: local slot %d out of %d locals at pc=%d",
					fn.Name, slot, fn.NumLocals, pc)
			}
		case OpCall:
			idx := int(binary.LittleEndian.Uint16(code[pc+1:]))
			if idx >= nFunc {
				return loadErr(ReasonInvalidFunction,
					"function %q: call target %d out of %d functions at pc=%d",
					fn.Name, idx, nFunc, pc)
			}
		case OpJmp, OpJmpIfFalse, OpJmpIfTrue, OpTry:
			target := int(binary.LittleEndian.Uint32(code[pc+1:]))
			if target >= len(code) {
				return loadErr(ReasonInvalidFunction,
					"function %q: jump target %d out of code size %d at pc=%d",
					fn.Name, target, len(code), pc)
			}
			jumps = append(jumps, jumpRef{at: pc, target: target})
		}
		pc += 1 + info.operandN
	}
	boundary[len(code)] = true
	for _, j := range jumps {
		if !boundary[j.target] {
			return loadErr(ReasonInvalidFunction,
				"function %q: jump target %d (from pc=%d) is not an instruction boundary",
				fn.Name, j.target, j.at)
		}
	}
	return nil
}

// Encode 将 Program 序列化为字节码制品（供工具与测试使用）。
func Encode(p *Program) ([]byte, error) {
	if p == nil {
		return nil, fmt.Errorf("encode: nil program")
	}
	out := []byte(Magic)
	out = binary.LittleEndian.AppendUint16(out, p.Major)
	out = binary.LittleEndian.AppendUint16(out, p.Minor)
	out = binary.LittleEndian.AppendUint16(out, 0) // flags

	out = binary.LittleEndian.AppendUint32(out, uint32(len(p.Consts)))
	for _, c := range p.Consts {
		switch c.Kind {
		case KindInt:
			out = append(out, constInt)
			out = binary.LittleEndian.AppendUint64(out, uint64(c.I))
		case KindStr:
			out = append(out, constStr)
			out = binary.LittleEndian.AppendUint32(out, uint32(len(c.S)))
			out = append(out, c.S...)
		default:
			return nil, fmt.Errorf("encode: unknown constant kind %d", c.Kind)
		}
	}

	out = binary.LittleEndian.AppendUint32(out, uint32(len(p.Funcs)))
	for _, f := range p.Funcs {
		out = binary.LittleEndian.AppendUint16(out, uint16(len(f.Name)))
		out = append(out, f.Name...)
		out = binary.LittleEndian.AppendUint16(out, uint16(f.NumParams))
		out = binary.LittleEndian.AppendUint16(out, uint16(f.NumLocals))
		out = binary.LittleEndian.AppendUint32(out, uint32(len(f.Code)))
		out = append(out, f.Code...)
	}

	out = binary.LittleEndian.AppendUint32(out, uint32(p.Entry))
	sum := crc32.ChecksumIEEE(out)
	out = binary.LittleEndian.AppendUint32(out, sum)
	return out, nil
}
