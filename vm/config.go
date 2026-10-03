package vm

// Config controls execution limits.
type Config struct {
	// MaxSteps is the maximum number of instructions executed. 0 means unlimited.
	MaxSteps uint64
	// MaxMemoryBytes is the maximum memory an instance may use. 0 means unlimited.
	MaxMemoryBytes int64
	// MaxCallDepth is the maximum recursion/call depth. 0 means unlimited.
	MaxCallDepth int
	// MaxStackDepth is the maximum operand stack depth per frame. 0 means unlimited.
	MaxStackDepth int
}

// DefaultConfig returns a configuration with sensible limits.
func DefaultConfig() Config {
	return Config{
		MaxSteps:       1_000_000,
		MaxMemoryBytes: 64 * 1024 * 1024, // 64 MiB
		MaxCallDepth:   256,
		MaxStackDepth:  1024,
	}
}

// Validate checks the configuration for consistency.
func (c *Config) Validate() error {
	if c.MaxSteps == 0 && c.MaxMemoryBytes == 0 && c.MaxCallDepth == 0 && c.MaxStackDepth == 0 {
		return nil // unlimited
	}
	return nil
}
