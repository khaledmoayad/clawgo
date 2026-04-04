//go:build !linux

package sandbox

import (
	"context"
	"fmt"
)

// BwrapSandbox is a stub on non-Linux platforms.
// Bubblewrap is only available on Linux.
type BwrapSandbox struct {
	AllowNetwork bool
}

func (s *BwrapSandbox) Type() SandboxType { return TypeBwrap }

// IsAvailable always returns false on non-Linux platforms.
func (s *BwrapSandbox) IsAvailable() bool { return false }

// Execute returns an error on non-Linux platforms.
func (s *BwrapSandbox) Execute(_ context.Context, _, _ string, _ []string) (*ExecuteResult, error) {
	return nil, fmt.Errorf("bubblewrap sandbox is only available on Linux")
}
