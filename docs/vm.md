# Virtual Machine Execution Semantics

This document describes how the stack-based virtual machine executes bytecode.

## Execution Model

The VM is a stack machine. Each function call creates a frame with:

- A private operand stack.
- Local variables (parameters + extra locals).
- A program counter (pc).
- A stack of active try blocks.

All frames share the module's read-only bytecode. Each execution instance has
its own frames, stacks, and locals, so instances are fully isolated.

## Operand Stack

Each frame has a private operand stack. Instructions push and pop values
according to their semantics. The VM checks:

- **Stack overflow**: pushing beyond `MaxStackDepth` aborts with
  `ErrStackOverflow`.
- **Stack underflow**: popping from an empty stack aborts with
  `ErrStackUnderflow`.

## Arithmetic and Type Handling

- Integers and floats: binary arithmetic promotes int to float if either
  operand is a float.
- String concatenation: `ADD` with at least one string operand concatenates.
- Division by zero: integer and float division by zero abort with
  `ErrDivisionByZero`.
- Comparison: same-kind comparisons are type-specific; cross-kind numeric
  comparisons promote to float. Ordering comparisons across non-numeric kinds
  abort with `ErrTypeMismatch`.

## Jumps

Jump targets are validated at execution time:

- The target must be within the function's code bounds.
- The target must point to a valid instruction boundary (not the middle of an
  instruction).

An invalid jump aborts with `ErrInvalidJumpTarget`.

## Function Calls and Recursion

- `CALL func_idx argc`: pops `argc` arguments from the current frame, creates
  a new frame, and binds the arguments to the new frame's parameters.
- `RET`: pops the current frame and pushes `null` to the caller.
- `RET_VALUE`: pops the return value, pops the current frame, and pushes the
  value to the caller.
- Returning from the entry function ends execution.

The call depth is bounded by `MaxCallDepth`. Exceeding it aborts with
`ErrCallDepthLimit`.

## Exception Handling

Exceptions are thrown with `THROW`, which pops the top of stack as the
exception value. If the value is not an exception, it is wrapped in a new
exception with its string representation.

### Try Blocks

- `TRY handler finally`: registers a try block. `handler` is the catch
  handler offset (-1 for none), `finally` is the cleanup offset (-1 for none).
- `END_TRY`: removes the current try block (normal completion).
- `END_FINALLY`: marks the end of a finally block.

### Stack Unwinding

When an exception is thrown, the VM unwinds the call stack:

1. For each frame (from most recent to oldest):
   a. Pop try blocks in reverse registration order (LIFO).
   b. If a try block has a catch handler, jump to the handler with the
      exception on the stack.
   c. If a try block has a finally block, run the finally code, then continue
      unwinding with the original exception.
2. If no handler is found, the exception is uncaught.

### Cleanup Rules

- Finally blocks run during unwinding in LIFO order (most recent first).
- If a finally block throws a new exception, the new exception replaces the
  original. The original is added to the new exception's suppressed list.
- If a finally block completes normally, unwinding continues with the
  original exception.

### Uncaught Exceptions

An uncaught exception aborts execution with `ErrUncaughtException`. The error
includes a call trace (frame names and offsets at the throw point).

## Limits

### Step Limit

`MaxSteps` bounds the number of instructions executed. Exceeding it aborts
with `ErrStepLimit`. This prevents infinite loops from running forever.

### Memory Limit

`MaxMemoryBytes` bounds the memory an instance may use. Memory is tracked as:

- 16 bytes per operand stack slot.
- 16 bytes per local slot.
- 16 + len bytes per runtime-created string.
- 48 + 32*trace_len bytes per exception.

Exceeding the limit aborts with `ErrMemoryLimit`.

### Determinism

Execution is deterministic: the same module, configuration, and inputs always
produce the same result and step count. The VM does not use randomness or
time-based behavior.

## Concurrency

A loaded module is read-only and safe for concurrent use. Multiple goroutines
can execute the same module simultaneously. Each execution creates an
independent instance with its own frames, stacks, and locals. There is no
shared mutable state between instances.

## Configuration

The `Config` struct controls execution limits:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| MaxSteps | uint64 | 1,000,000 | Maximum instructions (0 = unlimited) |
| MaxMemoryBytes | int64 | 64 MiB | Maximum memory (0 = unlimited) |
| MaxCallDepth | int | 256 | Maximum call depth (0 = unlimited) |
| MaxStackDepth | int | 1024 | Maximum operand stack depth per frame (0 = unlimited) |

## Error Types

### Load Errors

| Kind | Description |
|------|-------------|
| ErrBadMagic | Magic bytes do not match |
| ErrBadFormatVersion | Unsupported binary format version |
| ErrUnsupportedVersion | Unsupported bytecode version |
| ErrTruncated | Artifact is truncated |
| ErrBadConstantType | Unknown constant type tag |
| ErrConstantIndexOutOfRange | Constant index out of range |
| ErrBadOpcode | Unknown opcode |
| ErrOpcodeVersionMismatch | Opcode requires a newer version than declared |
| ErrLocalIndexOutOfRange | Local index out of range |
| ErrFunctionIndexOutOfRange | Function index out of range |
| ErrArgCountMismatch | Argument count mismatch |
| ErrInvalidCode | Invalid code structure |
| ErrEmptyModule | Module has no functions |

### Runtime Errors

| Kind | Description |
|------|-------------|
| ErrStackOverflow | Operand stack overflow |
| ErrStackUnderflow | Operand stack underflow |
| ErrInvalidJumpTarget | Invalid jump target |
| ErrTypeMismatch | Type mismatch in operation |
| ErrDivisionByZero | Division by zero |
| ErrCallDepthLimit | Call depth limit exceeded |
| ErrStepLimit | Step limit exceeded |
| ErrMemoryLimit | Memory limit exceeded |
| ErrUncaughtException | Uncaught exception |
| ErrInvalidOperation | Invalid operation |
| ErrLocalIndex | Local index out of range |
