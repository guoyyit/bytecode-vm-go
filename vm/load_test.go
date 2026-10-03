package vm

import (
	"testing"

	"github.com/guoyyit/bytecode-vm-go/bytecode"
)

// buildRawModule builds a module with a single function from raw code bytes.
func buildRawModule(major, minor uint16, code []byte) (*bytecode.Module, error) {
	w := bytecode.NewWriter(major, minor)
	w.AddFunction("main", 0, 0, code)
	data, err := w.Bytes()
	if err != nil {
		return nil, err
	}
	return bytecode.Load(data)
}

func TestLoadBadMagic(t *testing.T) {
	data := []byte{'X', 'Y', 'Z', 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	_, err := bytecode.Load(data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	le, ok := err.(*bytecode.LoadError)
	if !ok {
		t.Fatalf("expected LoadError, got %T", err)
	}
	if le.Kind != bytecode.ErrBadMagic {
		t.Fatalf("expected ErrBadMagic, got %v", le.Kind)
	}
}

func TestLoadBadFormatVersion(t *testing.T) {
	data := []byte{'B', 'V', 'M', 2, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	_, err := bytecode.Load(data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	le, ok := err.(*bytecode.LoadError)
	if !ok {
		t.Fatalf("expected LoadError, got %T", err)
	}
	if le.Kind != bytecode.ErrBadFormatVersion {
		t.Fatalf("expected ErrBadFormatVersion, got %v", le.Kind)
	}
}

func TestLoadUnsupportedMajorVersion(t *testing.T) {
	// Build a valid module then patch the major version to 2.
	w := bytecode.NewWriter(1, 0)
	w.AddFunction("main", 0, 0, []byte{byte(bytecode.OpNop), byte(bytecode.OpRetValue)})
	data, _ := w.Bytes()
	data[4] = 2 // major version
	_, err := bytecode.Load(data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	le, ok := err.(*bytecode.LoadError)
	if !ok {
		t.Fatalf("expected LoadError, got %T", err)
	}
	if le.Kind != bytecode.ErrUnsupportedVersion {
		t.Fatalf("expected ErrUnsupportedVersion, got %v", le.Kind)
	}
}

func TestLoadUnsupportedMinorVersion(t *testing.T) {
	w := bytecode.NewWriter(1, 0)
	w.AddFunction("main", 0, 0, []byte{byte(bytecode.OpNop), byte(bytecode.OpRetValue)})
	data, _ := w.Bytes()
	data[6] = 5 // minor version 5
	_, err := bytecode.Load(data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	le, ok := err.(*bytecode.LoadError)
	if !ok {
		t.Fatalf("expected LoadError, got %T", err)
	}
	if le.Kind != bytecode.ErrUnsupportedVersion {
		t.Fatalf("expected ErrUnsupportedVersion, got %v", le.Kind)
	}
}

func TestLoadUnknownOpcode(t *testing.T) {
	code := []byte{0xFF, byte(bytecode.OpRetValue)}
	_, err := buildRawModule(1, 0, code)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	le, ok := err.(*bytecode.LoadError)
	if !ok {
		t.Fatalf("expected LoadError, got %T", err)
	}
	if le.Kind != bytecode.ErrBadOpcode {
		t.Fatalf("expected ErrBadOpcode, got %v", le.Kind)
	}
}

func TestLoadTruncatedInstruction(t *testing.T) {
	// PUSH_INT needs 4 operand bytes; provide only 2.
	code := []byte{byte(bytecode.OpPushInt), 0x00, 0x01}
	_, err := buildRawModule(1, 0, code)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	le, ok := err.(*bytecode.LoadError)
	if !ok {
		t.Fatalf("expected LoadError, got %T", err)
	}
	if le.Kind != bytecode.ErrTruncated {
		t.Fatalf("expected ErrTruncated, got %v", le.Kind)
	}
}

func TestLoadEmptyModule(t *testing.T) {
	// Craft a module with zero functions: header (10) + const count (4) + func count (4).
	data := make([]byte, 18)
	copy(data[0:4], []byte{'B', 'V', 'M', 1})
	data[4] = 1 // major
	data[6] = 0 // minor
	// const count = 0 (offset 10-13)
	// func count = 0 (offset 14-17)
	_, err := bytecode.Load(data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	le, ok := err.(*bytecode.LoadError)
	if !ok {
		t.Fatalf("expected LoadError, got %T", err)
	}
	if le.Kind != bytecode.ErrEmptyModule {
		t.Fatalf("expected ErrEmptyModule, got %v", le.Kind)
	}
}

func TestLoadBadConstantType(t *testing.T) {
	// Craft a module with one constant of unknown type.
	// Header (10) + const count (4) + [type=99] + func count (4) + func entry...
	data := make([]byte, 10)
	copy(data[0:4], []byte{'B', 'V', 'M', 1})
	data[4] = 1 // major
	data[6] = 0 // minor
	// const count = 1 (offset 10-13)
	data = append(data, 1, 0, 0, 0)
	// constant type = 99 (offset 14)
	data = append(data, 99)
	// func count = 1 (offset 15-18)
	data = append(data, 1, 0, 0, 0)
	// func entry: nameLen(2) + numParams(1) + numLocals(1) + codeLen(4) + code
	data = append(data, 0, 0)               // nameLen = 0
	data = append(data, 0)                  // numParams = 0
	data = append(data, 0)                  // numLocals = 0
	data = append(data, 2, 0, 0, 0)         // codeLen = 2
	data = append(data, byte(bytecode.OpNop), byte(bytecode.OpRetValue))
	_, err := bytecode.Load(data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	le, ok := err.(*bytecode.LoadError)
	if !ok {
		t.Fatalf("expected LoadError, got %T", err)
	}
	if le.Kind != bytecode.ErrBadConstantType {
		t.Fatalf("expected ErrBadConstantType, got %v", le.Kind)
	}
}

func TestLoadConstantIndexOutOfRange(t *testing.T) {
	// PUSH_INT references const index 5, but pool is empty.
	code := []byte{byte(bytecode.OpPushInt), 0x05, 0x00, 0x00, 0x00, byte(bytecode.OpRetValue)}
	_, err := buildRawModule(1, 0, code)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	le, ok := err.(*bytecode.LoadError)
	if !ok {
		t.Fatalf("expected LoadError, got %T", err)
	}
	if le.Kind != bytecode.ErrConstantIndexOutOfRange {
		t.Fatalf("expected ErrConstantIndexOutOfRange, got %v", le.Kind)
	}
}

func TestLoadLocalIndexOutOfRange(t *testing.T) {
	// STORE_LOCAL references local index 5, but numLocals is 0.
	code := []byte{byte(bytecode.OpStoreLocal), 0x05, byte(bytecode.OpRetValue)}
	_, err := buildRawModule(1, 0, code)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	le, ok := err.(*bytecode.LoadError)
	if !ok {
		t.Fatalf("expected LoadError, got %T", err)
	}
	if le.Kind != bytecode.ErrLocalIndexOutOfRange {
		t.Fatalf("expected ErrLocalIndexOutOfRange, got %v", le.Kind)
	}
}

func TestLoadFunctionIndexOutOfRange(t *testing.T) {
	// CALL references function index 5, but only 1 function exists.
	code := []byte{byte(bytecode.OpCall), 0x05, 0x00, 0x00, 0x00, 0x00, byte(bytecode.OpRetValue)}
	_, err := buildRawModule(1, 0, code)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	le, ok := err.(*bytecode.LoadError)
	if !ok {
		t.Fatalf("expected LoadError, got %T", err)
	}
	if le.Kind != bytecode.ErrFunctionIndexOutOfRange {
		t.Fatalf("expected ErrFunctionIndexOutOfRange, got %v", le.Kind)
	}
}

func TestLoadBadJumpTarget(t *testing.T) {
	// Jump targets are validated at execution time, not load time.
	// A module with a bad jump target should load successfully.
	code := []byte{byte(bytecode.OpJmp), 0xFF, 0xFF, 0xFF, 0xFF, byte(bytecode.OpRetValue)}
	mod, err := buildRawModule(1, 0, code)
	if err != nil {
		t.Fatalf("module should load (jump validation is at runtime): %v", err)
	}
	if mod == nil {
		t.Fatal("expected non-nil module")
	}
}

func TestLoadArgCountMismatch(t *testing.T) {
	// Function main takes 0 params, but CALL passes 2 args.
	code := []byte{byte(bytecode.OpCall), 0x00, 0x00, 0x00, 0x00, 0x02, byte(bytecode.OpRetValue)}
	_, err := buildRawModule(1, 0, code)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	le, ok := err.(*bytecode.LoadError)
	if !ok {
		t.Fatalf("expected LoadError, got %T", err)
	}
	if le.Kind != bytecode.ErrArgCountMismatch {
		t.Fatalf("expected ErrArgCountMismatch, got %v", le.Kind)
	}
}
