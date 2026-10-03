# Bytecode Virtual Machine

A stack-based virtual machine implemented in Go. It loads and executes custom
bytecode artifacts entirely on the local machine, with no external services or
credentials.

## Features

- **Bytecode format** with explicit versioning, constant pool, and function
  table. Load-time validation rejects corrupt structures, unsupported
  versions, and unknown opcodes with distinguishable error reasons.
- **Operand stack execution** covering arithmetic, comparison, bitwise
  operations, conditional and unconditional jumps. Stack overflow/underflow
  and illegal jump targets are detected at execution time.
- **Function calls and recursion** with parameter passing, local variables,
  and return values. Call depth is bounded by a configurable limit.
- **Exception handling** with try/catch/finally blocks and deterministic
  stack unwinding. Cleanup logic runs in LIFO order; cleanup that throws
  follows clear rules. Uncaught exceptions carry a readable call trace.
- **Configurable limits** on instruction steps and memory. Execution stops
  deterministically when limits are exceeded, with consistent results and
  statistics.
- **Concurrency**: loaded bytecode is read-only and can be used by multiple
  execution instances concurrently. Each instance has isolated state.
- **Backward compatibility**: the format supports versioned evolution. Old
  bytecode loads and runs correctly on newer VMs; incompatible inputs are
  rejected at load time.

## Quick Start

### Build

```bash
go build ./...
```

### Generate Example Bytecode

```bash
go run ./examples/gen ./examples/out
```

This generates example bytecode files (factorial, fibonacci, exception demo,
pow demo) in `./examples/out`.

### Run

```bash
# Run factorial(5)
go run ./cmd/bvm run ./examples/out/factorial.bvm --entry factorial --args 5

# Run fibonacci(10)
go run ./cmd/bvm run ./examples/out/fibonacci.bvm --entry fib --args 10

# Run the exception demo
go run ./cmd/bvm run ./examples/out/exception.bvm --entry main

# Run the pow demo (v1.1 bytecode)
go run ./cmd/bvm run ./examples/out/pow.bvm --entry main
```

### Inspect

```bash
go run ./cmd/bvm info ./examples/out/factorial.bvm
```

## CLI Reference

### `bvm run <file> [flags]`

| Flag | Default | Description |
|------|---------|-------------|
| `--entry` | `main` | Entry function name |
| `--args` | — | Comma-separated integer arguments |
| `--max-steps` | 1,000,000 | Maximum instruction steps |
| `--max-memory` | 64 MiB | Maximum memory in bytes |
| `--max-depth` | 256 | Maximum call depth |
| `--max-stack` | 1024 | Maximum operand stack depth |

### `bvm info <file>`

Displays bytecode version, constant pool, and function table.

## Programmatic API

```go
import (
    "github.com/guoyyit/bytecode-vm-go/bytecode"
    "github.com/guoyyit/bytecode-vm-go/vm"
)

// Load a bytecode artifact.
mod, err := bytecode.Load(data)

// Execute with default configuration.
result, err := vm.Execute(mod, vm.DefaultConfig(), "main", nil)

// Execute with custom limits.
cfg := vm.Config{
    MaxSteps:       10000,
    MaxMemoryBytes: 1024 * 1024,
    MaxCallDepth:   100,
    MaxStackDepth:  256,
}
result, err := vm.Execute(mod, cfg, "main", nil)
```

## Building Bytecode

Use the `bytecode.Writer` and `bytecode.CodeBuilder` to construct artifacts:

```go
w := bytecode.NewWriter(1, 0) // version 1.0
one := w.AddIntConstant(1)

cb := bytecode.NewCodeBuilder()
cb.EmitOpU8(bytecode.OpLoadLocal, 0)
cb.EmitOpU32(bytecode.OpPushInt, one)
cb.EmitOp(bytecode.OpAdd)
cb.EmitOp(bytecode.OpRetValue)
code, _ := cb.Code()

w.AddFunction("inc", 1, 1, code)
data, _ := w.Bytes()
```

## Documentation

- [Bytecode Format Specification](docs/bytecode.md)
- [VM Execution Semantics](docs/vm.md)

## Testing

```bash
go test ./...
```

Run with the race detector to verify concurrency safety:

```bash
go test -race ./...
```

## Project Structure

```
bytecode/          Bytecode format, encoder, decoder, load validation
  format.go        Format constants and opcode definitions
  module.go        Module and Function types
  encode.go        Writer and CodeBuilder
  decode.go        Reader with structural validation
  errors.go        Load error types
vm/                Virtual machine
  value.go         Runtime values
  errors.go        Runtime error types
  exception.go     Exception type
  config.go        Configuration
  frame.go         Call frame
  instance.go      Execution instance and run loop
  exec.go          Instruction execution
cmd/bvm/           CLI tool
examples/gen/      Example bytecode generator
docs/              Documentation
```
