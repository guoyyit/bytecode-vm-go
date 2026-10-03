package vm

import "fmt"

// ValueKind 是栈上值的类型标签。
type ValueKind byte

const (
	KindInt ValueKind = iota
	KindStr
)

// Value 是虚拟机操作的值。
type Value struct {
	Kind ValueKind
	I    int64
	S    string
}

// Int 构造整数值。
func Int(v int64) Value { return Value{Kind: KindInt, I: v} }

// Str 构造字符串值。
func Str(s string) Value { return Value{Kind: KindStr, S: s} }

func (v Value) String() string {
	if v.Kind == KindStr {
		return fmt.Sprintf("%q", v.S)
	}
	return fmt.Sprintf("%d", v.I)
}

// truthy 报告值在条件跳转中是否为真（非零整数 / 非空字符串）。
func (v Value) truthy() bool {
	if v.Kind == KindStr {
		return v.S != ""
	}
	return v.I != 0
}
