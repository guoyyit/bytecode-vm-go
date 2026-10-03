# bytecode-vm-go

一套纯本机运行的栈式字节码虚拟机（Go 实现，无任何外部依赖）。负责加载、校验并执行自定义二进制字节码制品，支持函数调用与递归、异常块与栈展开、可配置的资源上限，并保证加载后的制品可被多个执行实例并发安全地共享。

## 快速开始

```go
data, _ := os.ReadFile("program.bc")
prog, err := vm.Load(data)          // 加载并完整校验；失败返回 *vm.LoadError
if err != nil { /* 按 err.(*vm.LoadError).Reason 区分原因 */ }

machine := vm.New(prog, vm.Config{  // 每个执行实例独立；prog 只读共享
    MaxSteps:     1_000_000,
    MaxMemory:    1 << 20,
    MaxCallDepth: 256,
    MaxStack:     1024,
})
result, stats, err := machine.Run(vm.Int(10)) // 失败返回 *vm.RuntimeError（含调用轨迹）
```

## 字节码制品结构

全部多字节字段为小端序：

```
magic        4B   "BCVM"
major        u16  主版本（当前为 1，不同即拒绝）
minor        u16  次版本（当前支持 <= 1）
flags        u16  保留，必须为 0
const_count  u32
consts       ...  每项：tag u8（0=int64，1=string）；int 为 8B，string 为 u32 长度 + 字节
func_count   u32  必须 >= 1
funcs        ...  每项：nameLen u16 + name | numParams u16 | numLocals u16 | codeLen u32 | code
entry        u32  入口函数索引
crc32        u32  对以上全部字节的 IEEE 校验和
```

指令为定长编码：1 字节操作码 + 定长操作数（u16 索引或 u32 地址）。跳转目标为**函数内绝对地址**，且必须落在指令边界上。

### 加载期校验（失败即整体拒绝，不产生部分可用制品）

| 失败原因 (`LoadError.Reason`) | 触发条件 |
|---|---|
| `ReasonBadMagic` | magic 不是 `BCVM` |
| `ReasonBadChecksum` | CRC32 不匹配（结构损坏） |
| `ReasonTruncated` | 任何字段读取越界 |
| `ReasonUnsupportedVersion` | 主版本不同 / 次版本过高 / 未知 flags / 制品使用了高于其声明次版本的操作码 |
| `ReasonUnknownOpcode` | 未注册的操作码 |
| `ReasonInvalidConstant` | 非法常量标签、常量索引越界 |
| `ReasonInvalidFunction` | 函数表结构非法、局部变量槽越界、调用目标越界、跳转目标越界或不在指令边界 |
| `ReasonInvalidEntry` | 入口索引越界 |

## 指令集

| 类别 | 指令（操作数） |
|---|---|
| 常量/变量 | `const u16`、`load u16`、`store u16`、`pop`、`dup`、`nop` |
| 算术 | `add` `sub` `mul` `div` `mod` `neg`（int64；除零报错） |
| 位运算 | `and` `or` `xor` `not`；`shl` `shr`（v1.1 引入） |
| 比较 | `eq` `ne` `lt` `le` `gt` `ge`（结果 0/1） |
| 跳转 | `jmp u32`、`jmp_if_false u32`、`jmp_if_true u32` |
| 调用 | `call u16`（参数从栈弹出，数量取被调方 `numParams`）、`ret` |
| 异常 | `try u32`（注册处理入口）、`try_end`（注销）、`throw`（弹出栈顶作为异常值） |
| 控制 | `halt`（以栈顶为结果停机） |

## 执行语义

- **操作数栈**：每帧独立。`push` 超出 `MaxStack` 报 `KindStackOverflow`；空栈弹出报 `KindStackUnderflow`。
- **调用**：`call` 新建帧，参数写入被调方局部变量槽 `0..numParams-1`；`ret` 以当前栈顶（空栈则为零值）作为返回值压回调用方。深度超过 `MaxCallDepth` 报 `KindCallDepthExceeded`。
- **异常与展开**：`throw` 自内向外查找已注册的 `try` 块；命中时操作数栈恢复到登记时深度、异常值入栈、跳转到处理入口。清理逻辑（处理块）按注册逆序（内层优先）执行；**处理逻辑自身再次抛出时，该处理器已被弹出，异常继续向外层传播**（处理器不捕获自身异常）。`try_end` 注销当前处理器。无任何处理器时报 `KindUncaughtException`，错误携带自内向外的完整调用轨迹（`RuntimeError.Trace`）。
- **运行时错误**（`*RuntimeError.Kind`）：`KindStackOverflow` `KindStackUnderflow` `KindBadJump` `KindCallDepthExceeded` `KindStepLimitExceeded` `KindMemoryLimitExceeded` `KindDivByZero` `KindTypeError` `KindInvalidLocal` `KindBadCall` `KindUncaughtException`。任何错误都中止执行，不会带着错误状态继续运行。

## 配置项（`vm.Config`，零值取默认值）

| 字段 | 默认 | 含义 |
|---|---|---|
| `MaxSteps` | 1,000,000 | 单次执行指令步数上限，超限报 `KindStepLimitExceeded` |
| `MaxMemory` | 1,048,576 | 内存上限（单位：槽位；含操作数栈、局部变量、帧与异常处理器开销），超限报 `KindMemoryLimitExceeded` |
| `MaxCallDepth` | 256 | 调用深度上限 |
| `MaxStack` | 1024 | 单帧操作数栈深度上限 |

超限即确定性停止；实例状态为内部私有，错误不会留下对外可见的半执行状态。固定配置与相同输入下，结果与 `Stats`（指令数、栈/内存峰值、调用次数）完全一致。

## 并发与隔离

`Program` 加载后不可变，可被任意多个 `VM` 实例并发共享；每个 `VM` 持有独立的帧栈、内存记账与统计，实例间无共享可变状态（`go test -race` 验证）。

## 版本演进

- 主版本不同：直接拒绝（不兼容变更）。
- 次版本向后兼容：运行时支持 `minor <= 当前次版本` 的制品；旧制品行为保持一致。
- 新操作码标注引入版本（如 `shl/shr/not` 自 v1.1 起）；声明较低次版本的制品若使用新操作码，在加载期以 `ReasonUnsupportedVersion` 拒绝。

## 测试

```sh
go test -race ./...
```

覆盖：加载校验（magic/校验和/截断/版本/未知操作码/各类索引越界）、算术与跳转、栈上下溢、递归与调用深度、异常捕获/嵌套重抛/跨帧展开/未捕获轨迹、步数与内存上限、版本兼容、并发实例隔离与确定性。
