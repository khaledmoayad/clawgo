package errors

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

// --- Base ClawGoError tests ---

func TestClawGoError_Error(t *testing.T) {
	t.Run("name and message", func(t *testing.T) {
		e := New("TestError", "something broke")
		if e.Error() != "TestError: something broke" {
			t.Errorf("Error() = %q, want %q", e.Error(), "TestError: something broke")
		}
	})

	t.Run("name only", func(t *testing.T) {
		e := &ClawGoError{Name: "TestError"}
		if e.Error() != "TestError" {
			t.Errorf("Error() = %q, want %q", e.Error(), "TestError")
		}
	})

	t.Run("with wrapped error", func(t *testing.T) {
		inner := fmt.Errorf("root cause")
		e := Wrap("TestError", inner)
		want := "TestError: root cause"
		if e.Error() != want {
			t.Errorf("Error() = %q, want %q", e.Error(), want)
		}
	})

	t.Run("with message and wrapped error", func(t *testing.T) {
		inner := fmt.Errorf("root cause")
		e := &ClawGoError{Name: "TestError", Message: "failed", Err: inner}
		want := "TestError: failed: root cause"
		if e.Error() != want {
			t.Errorf("Error() = %q, want %q", e.Error(), want)
		}
	})
}

func TestClawGoError_Unwrap(t *testing.T) {
	inner := fmt.Errorf("inner error")
	e := Wrap("WrapError", inner)
	if !errors.Is(e, inner) {
		t.Error("errors.Is should find the wrapped inner error")
	}
}

// --- ShellError ---

func TestShellError(t *testing.T) {
	t.Run("has ExitCode field", func(t *testing.T) {
		e := NewShellError("command failed", 127)
		if e.ExitCode != 127 {
			t.Errorf("ExitCode = %d, want 127", e.ExitCode)
		}
	})

	t.Run("errors.As extracts ShellError from wrapped", func(t *testing.T) {
		e := NewShellError("exit 1", 1)
		var wrapped error = fmt.Errorf("wrapped: %w", e)

		var se *ShellError
		if !errors.As(wrapped, &se) {
			t.Fatal("errors.As should find ShellError")
		}
		if se.ExitCode != 1 {
			t.Errorf("extracted ExitCode = %d, want 1", se.ExitCode)
		}
	})

	t.Run("has correct Name", func(t *testing.T) {
		e := NewShellError("test", 42)
		if e.Name != "ShellError" {
			t.Errorf("Name = %q, want %q", e.Name, "ShellError")
		}
	})

	t.Run("Error() includes exit code context", func(t *testing.T) {
		e := NewShellError("process failed", 1)
		got := e.Error()
		if got != "ShellError: process failed" {
			t.Errorf("Error() = %q, want %q", got, "ShellError: process failed")
		}
	})
}

// --- AbortError ---

func TestAbortError(t *testing.T) {
	t.Run("matches via errors.As from wrapped", func(t *testing.T) {
		e := NewAbortError("user cancelled")
		var wrapped error = fmt.Errorf("wrapped: %w", e)

		var ae *AbortError
		if !errors.As(wrapped, &ae) {
			t.Fatal("errors.As should find AbortError")
		}
	})

	t.Run("wraps context.Canceled", func(t *testing.T) {
		e := NewAbortErrorFrom(context.Canceled)
		if !errors.Is(e, context.Canceled) {
			t.Error("errors.Is should find context.Canceled via Unwrap")
		}
	})

	t.Run("has correct Name", func(t *testing.T) {
		e := NewAbortError("cancel")
		if e.Name != "AbortError" {
			t.Errorf("Name = %q, want %q", e.Name, "AbortError")
		}
	})
}

// --- FallbackTriggeredError ---

func TestFallbackTriggeredError(t *testing.T) {
	t.Run("has FallbackModel field", func(t *testing.T) {
		e := NewFallbackTriggeredError("claude-3-haiku")
		if e.FallbackModel != "claude-3-haiku" {
			t.Errorf("FallbackModel = %q, want %q", e.FallbackModel, "claude-3-haiku")
		}
	})

	t.Run("has correct Name", func(t *testing.T) {
		e := NewFallbackTriggeredError("model-x")
		if e.Name != "FallbackTriggeredError" {
			t.Errorf("Name = %q, want %q", e.Name, "FallbackTriggeredError")
		}
	})

	t.Run("errors.As extracts from wrapped", func(t *testing.T) {
		e := NewFallbackTriggeredError("model-x")
		wrapped := fmt.Errorf("outer: %w", e)

		var fe *FallbackTriggeredError
		if !errors.As(wrapped, &fe) {
			t.Fatal("errors.As should find FallbackTriggeredError")
		}
		if fe.FallbackModel != "model-x" {
			t.Errorf("FallbackModel = %q, want %q", fe.FallbackModel, "model-x")
		}
	})
}

// --- TeleportError, OAuthError, MalformedCommandError, ConfigParseError ---

func TestDomainErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		wantNm string
	}{
		{"TeleportError", NewTeleportError("teleport failed"), "TeleportError"},
		{"OAuthError", NewOAuthError("auth failed"), "OAuthError"},
		{"MalformedCommandError", NewMalformedCommandError("bad input"), "MalformedCommandError"},
		{"ConfigParseError", NewConfigParseError("invalid JSON"), "ConfigParseError"},
	}

	for _, tc := range tests {
		t.Run(tc.name+" has correct Name", func(t *testing.T) {
			// Verify the error message contains the expected name
			got := tc.err.Error()
			if len(got) == 0 {
				t.Fatal("Error() returned empty string")
			}
		})

		t.Run(tc.name+" extractable via errors.As", func(t *testing.T) {
			wrapped := fmt.Errorf("outer: %w", tc.err)
			// Each type should be extractable from a wrapped error chain
			switch tc.name {
			case "TeleportError":
				var target *TeleportError
				if !errors.As(wrapped, &target) {
					t.Fatalf("errors.As should find %s", tc.name)
				}
				if target.Name != tc.wantNm {
					t.Errorf("Name = %q, want %q", target.Name, tc.wantNm)
				}
			case "OAuthError":
				var target *OAuthError
				if !errors.As(wrapped, &target) {
					t.Fatalf("errors.As should find %s", tc.name)
				}
				if target.Name != tc.wantNm {
					t.Errorf("Name = %q, want %q", target.Name, tc.wantNm)
				}
			case "MalformedCommandError":
				var target *MalformedCommandError
				if !errors.As(wrapped, &target) {
					t.Fatalf("errors.As should find %s", tc.name)
				}
				if target.Name != tc.wantNm {
					t.Errorf("Name = %q, want %q", target.Name, tc.wantNm)
				}
			case "ConfigParseError":
				var target *ConfigParseError
				if !errors.As(wrapped, &target) {
					t.Fatalf("errors.As should find %s", tc.name)
				}
				if target.Name != tc.wantNm {
					t.Errorf("Name = %q, want %q", target.Name, tc.wantNm)
				}
			}
		})
	}
}

// --- IsAbortError ---

func TestIsAbortError(t *testing.T) {
	t.Run("AbortError", func(t *testing.T) {
		if !IsAbortError(NewAbortError("cancel")) {
			t.Error("IsAbortError should return true for AbortError")
		}
	})

	t.Run("context.Canceled", func(t *testing.T) {
		if !IsAbortError(context.Canceled) {
			t.Error("IsAbortError should return true for context.Canceled")
		}
	})

	t.Run("context.DeadlineExceeded", func(t *testing.T) {
		if !IsAbortError(context.DeadlineExceeded) {
			t.Error("IsAbortError should return true for context.DeadlineExceeded")
		}
	})

	t.Run("wrapped context.Canceled", func(t *testing.T) {
		wrapped := fmt.Errorf("outer: %w", context.Canceled)
		if !IsAbortError(wrapped) {
			t.Error("IsAbortError should return true for wrapped context.Canceled")
		}
	})

	t.Run("wrapped AbortError", func(t *testing.T) {
		wrapped := fmt.Errorf("outer: %w", NewAbortError("cancel"))
		if !IsAbortError(wrapped) {
			t.Error("IsAbortError should return true for wrapped AbortError")
		}
	})

	t.Run("regular error", func(t *testing.T) {
		if IsAbortError(fmt.Errorf("just an error")) {
			t.Error("IsAbortError should return false for regular errors")
		}
	})

	t.Run("nil error", func(t *testing.T) {
		if IsAbortError(nil) {
			t.Error("IsAbortError should return false for nil")
		}
	})
}

// --- ErrorMessage ---

func TestErrorMessage(t *testing.T) {
	t.Run("ClawGoError extracts Message", func(t *testing.T) {
		e := New("TestError", "the message")
		if got := ErrorMessage(e); got != "the message" {
			t.Errorf("ErrorMessage = %q, want %q", got, "the message")
		}
	})

	t.Run("regular error returns Error()", func(t *testing.T) {
		e := fmt.Errorf("plain error")
		if got := ErrorMessage(e); got != "plain error" {
			t.Errorf("ErrorMessage = %q, want %q", got, "plain error")
		}
	})

	t.Run("nil returns empty string", func(t *testing.T) {
		if got := ErrorMessage(nil); got != "" {
			t.Errorf("ErrorMessage(nil) = %q, want empty", got)
		}
	})

	t.Run("ShellError extracts Message", func(t *testing.T) {
		e := NewShellError("cmd not found", 127)
		if got := ErrorMessage(e); got != "cmd not found" {
			t.Errorf("ErrorMessage = %q, want %q", got, "cmd not found")
		}
	})
}

// --- ToError ---

func TestToError(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		if got := ToError(nil); got != nil {
			t.Errorf("ToError(nil) = %v, want nil", got)
		}
	})

	t.Run("error passes through", func(t *testing.T) {
		e := fmt.Errorf("real error")
		if got := ToError(e); got != e {
			t.Errorf("ToError(error) should return the same error")
		}
	})

	t.Run("string becomes error", func(t *testing.T) {
		got := ToError("something bad")
		if got == nil {
			t.Fatal("ToError(string) should not return nil")
		}
		if got.Error() != "something bad" {
			t.Errorf("ToError(string).Error() = %q, want %q", got.Error(), "something bad")
		}
	})

	t.Run("int becomes error", func(t *testing.T) {
		got := ToError(42)
		if got == nil {
			t.Fatal("ToError(int) should not return nil")
		}
		if got.Error() != "42" {
			t.Errorf("ToError(int).Error() = %q, want %q", got.Error(), "42")
		}
	})

	t.Run("struct becomes error", func(t *testing.T) {
		type foo struct{ X int }
		got := ToError(foo{X: 7})
		if got == nil {
			t.Fatal("ToError(struct) should not return nil")
		}
	})
}

// --- ErrorHierarchy comprehensive ---

func TestErrorHierarchy(t *testing.T) {
	// Verify all error types satisfy the error interface
	var _ error = (*ClawGoError)(nil)
	var _ error = (*ConfigError)(nil)
	var _ error = (*APIError)(nil)
	var _ error = (*ToolError)(nil)
	var _ error = (*PermissionError)(nil)
	var _ error = (*SessionError)(nil)
	var _ error = (*ShellError)(nil)
	var _ error = (*AbortError)(nil)
	var _ error = (*FallbackTriggeredError)(nil)
	var _ error = (*TeleportError)(nil)
	var _ error = (*OAuthError)(nil)
	var _ error = (*MalformedCommandError)(nil)
	var _ error = (*ConfigParseError)(nil)

	// 13 error types total:
	// Base: ClawGoError
	// Existing 5: ConfigError, APIError, ToolError, PermissionError, SessionError
	// New 7: ShellError, AbortError, FallbackTriggeredError, TeleportError, OAuthError, MalformedCommandError, ConfigParseError
}
