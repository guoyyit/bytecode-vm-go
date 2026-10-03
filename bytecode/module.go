package bytecode

// ValueKind identifies the kind of a constant pool value.
type ValueKind uint8

const (
	VInt    ValueKind = 1
	VFloat  ValueKind = 2
	VString ValueKind = 3
)

// Value is a constant pool entry. Only int, float, and string are stored in
// the constant pool; booleans and null are pushed via dedicated opcodes.
type Value struct {
	Kind ValueKind
	Int  int64
	Float float64
	Str  string
}

// Function describes a single function in a module.
type Function struct {
	Name      string
	NumParams uint8
	NumLocals uint8 // total locals including params
	Code      []byte

	// validInstStarts marks code offsets that begin a valid instruction.
	// Computed and validated at load time. Used to validate jump targets.
	validInstStarts []bool
}

// ValidInstStart reports whether off is the start of a valid instruction.
func (f *Function) ValidInstStart(off int) bool {
	if off < 0 || off >= len(f.validInstStarts) {
		return false
	}
	return f.validInstStarts[off]
}

// Module is a loaded, validated bytecode artifact. It is read-only after
// loading and safe for concurrent use by multiple VM instances.
type Module struct {
	FormatVersion uint16
	MajorVersion  uint16
	MinorVersion  uint16
	Flags         uint16
	Constants     []Value
	Functions     []*Function
}

// FunctionCount returns the number of functions.
func (m *Module) FunctionCount() int { return len(m.Functions) }

// ConstantCount returns the number of constants.
func (m *Module) ConstantCount() int { return len(m.Constants) }

// VersionString returns a human-readable version.
func (m *Module) VersionString() string {
	return formatVersionString(m.MajorVersion, m.MinorVersion)
}

func formatVersionString(major, minor uint16) string {
	return string(rune('0'+major)) + "." + string(rune('0'+minor))
}
