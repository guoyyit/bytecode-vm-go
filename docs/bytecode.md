# Bytecode Format Specification

This document describes the binary format of bytecode artifacts used by the
stack-based virtual machine.

## Overview

A bytecode artifact is a self-contained binary file containing:

1. A header with magic bytes and version information.
2. A constant pool (integers, floats, strings).
3. A function table, where each function has its own code section.

The format is designed for forward compatibility: newer VMs can load older
artifacts, and the loader rejects artifacts that are too new or structurally
invalid.

## Binary Layout

All multi-byte integers are little-endian.

```
Offset  Size  Field
0       4     Magic = {'B','V','M', FormatVersion}
4       2     MajorVersion (uint16)
6       2     MinorVersion (uint16)
8       2     Flags (uint16)
10      4     ConstantPoolCount (uint32)
14      ...   ConstantPool entries
...     4     FunctionCount (uint32)
...     ...   Function entries
```

### Header

| Field | Size | Description |
|-------|------|-------------|
| Magic | 4 | `0x42 0x56 0x4D 0x01` ("BVM" + format version 1) |
| MajorVersion | 2 | Bytecode major version (currently 1) |
| MinorVersion | 2 | Bytecode minor version (0 or 1) |
| Flags | 2 | Reserved for future use |

### Constant Pool

Each entry has a 1-byte type tag followed by the value:

| Type | Tag | Value encoding |
|------|-----|----------------|
| Integer | 1 | int64 (8 bytes) |
| Float | 2 | float64 bits (8 bytes) |
| String | 3 | uint32 length + UTF-8 bytes |

### Function Table

Each function entry:

| Field | Size | Description |
|-------|------|-------------|
| NameLength | 2 | Length of the function name |
| Name | N | UTF-8 function name |
| NumParams | 1 | Number of parameters |
| NumLocals | 1 | Total locals (including params) |
| CodeLength | 4 | Length of the code section |
| Code | M | Bytecode instructions |

## Instruction Set

Each instruction is 1 opcode byte followed by fixed-size operands.

### v1.0 Instructions

| Opcode | Name | Operands | Description |
|--------|------|----------|-------------|
| 0x00 | NOP | — | No operation |
| 0x01 | PUSH_INT | u32 const | Push int constant |
| 0x02 | PUSH_FLOAT | u32 const | Push float constant |
| 0x03 | PUSH_STRING | u32 const | Push string constant |
| 0x04 | PUSH_TRUE | — | Push boolean true |
| 0x05 | PUSH_FALSE | — | Push boolean false |
| 0x06 | PUSH_NULL | — | Push null |
| 0x10 | POP | — | Pop top of stack |
| 0x11 | DUP | — | Duplicate top of stack |
| 0x20 | ADD | — | Add (int/float/string concat) |
| 0x21 | SUB | — | Subtract |
| 0x22 | MUL | — | Multiply |
| 0x23 | DIV | — | Divide |
| 0x24 | MOD | — | Modulo |
| 0x25 | NEG | — | Negate |
| 0x26 | EQ | — | Equal |
| 0x27 | NE | — | Not equal |
| 0x28 | LT | — | Less than |
| 0x29 | LE | — | Less than or equal |
| 0x2A | GT | — | Greater than |
| 0x2B | GE | — | Greater than or equal |
| 0x30 | AND | — | Bitwise and / logical and |
| 0x31 | OR | — | Bitwise or / logical or |
| 0x32 | NOT | — | Bitwise not / logical not |
| 0x40 | JMP | i32 target | Unconditional jump |
| 0x41 | JMP_IF_TRUE | i32 target | Jump if top is truthy |
| 0x42 | JMP_IF_FALSE | i32 target | Jump if top is falsy |
| 0x50 | LOAD_LOCAL | u8 idx | Push local variable |
| 0x51 | STORE_LOCAL | u8 idx | Store to local variable |
| 0x60 | CALL | u32 func, u8 argc | Call function |
| 0x61 | RET | — | Return null |
| 0x62 | RET_VALUE | — | Return top of stack |
| 0x70 | THROW | — | Throw top as exception |
| 0x71 | TRY | i32 handler, i32 finally | Register try block |
| 0x72 | END_TRY | — | End try block (normal) |
| 0x73 | END_FINALLY | — | End finally block |

### v1.1 Instructions

| Opcode | Name | Operands | Description |
|--------|------|----------|-------------|
| 0x80 | POW | — | Exponentiation |

## Version Compatibility

The loader enforces the following rules:

- The artifact's major version must be 1.
- The minor version must be in the range [0, 1].
- Each opcode used in the code must have been introduced in a version <= the
  artifact's declared version. For example, a v1.0 artifact using POW (a v1.1
  opcode) is rejected.

This ensures that:

- Old artifacts load and run correctly on newer VMs (backward compatibility).
- New artifacts are rejected by older VMs that cannot support them.
- Inconsistent artifacts (declaring an older version but using newer opcodes)
  are rejected at load time.

## Load-Time Validation

The loader performs the following structural checks:

- Magic bytes and format version.
- Version compatibility.
- Constant pool type tags and value ranges.
- Function count (must be non-zero).
- Function name, parameter, and local counts.
- Code length and instruction decoding (no truncated instructions).
- Opcode validity and version compatibility.
- Constant index references.
- Local index references.
- Call target and argument count.

Jump targets are validated at execution time (not load time), so the VM can
detect and report illegal jumps during execution.
