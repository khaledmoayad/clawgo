package tools

import (
	"testing"
)

func TestSemanticBoolean_BoolTrue(t *testing.T) {
	result, err := SemanticBoolean(true)
	if err != nil {
		t.Fatalf("SemanticBoolean(true) error: %v", err)
	}
	if !result {
		t.Error("SemanticBoolean(true) = false, want true")
	}
}

func TestSemanticBoolean_BoolFalse(t *testing.T) {
	result, err := SemanticBoolean(false)
	if err != nil {
		t.Fatalf("SemanticBoolean(false) error: %v", err)
	}
	if result {
		t.Error("SemanticBoolean(false) = true, want false")
	}
}

func TestSemanticBoolean_StringTrue(t *testing.T) {
	result, err := SemanticBoolean("true")
	if err != nil {
		t.Fatalf("SemanticBoolean(\"true\") error: %v", err)
	}
	if !result {
		t.Error("SemanticBoolean(\"true\") = false, want true")
	}
}

func TestSemanticBoolean_StringFalse(t *testing.T) {
	result, err := SemanticBoolean("false")
	if err != nil {
		t.Fatalf("SemanticBoolean(\"false\") error: %v", err)
	}
	if result {
		t.Error("SemanticBoolean(\"false\") = true, want false")
	}
}

func TestSemanticBoolean_StringYes_Rejects(t *testing.T) {
	_, err := SemanticBoolean("yes")
	if err == nil {
		t.Error("SemanticBoolean(\"yes\") should return error")
	}
}

func TestSemanticBoolean_StringTRUE_Rejects(t *testing.T) {
	// Only exact "true"/"false" accepted, not "TRUE"
	_, err := SemanticBoolean("TRUE")
	if err == nil {
		t.Error("SemanticBoolean(\"TRUE\") should return error")
	}
}

func TestSemanticBoolean_String1_Rejects(t *testing.T) {
	_, err := SemanticBoolean("1")
	if err == nil {
		t.Error("SemanticBoolean(\"1\") should return error")
	}
}

func TestSemanticBoolean_Nil_Rejects(t *testing.T) {
	_, err := SemanticBoolean(nil)
	if err == nil {
		t.Error("SemanticBoolean(nil) should return error")
	}
}

func TestSemanticBoolean_Int_Rejects(t *testing.T) {
	_, err := SemanticBoolean(42)
	if err == nil {
		t.Error("SemanticBoolean(42) should return error")
	}
}

func TestSemanticNumber_Int(t *testing.T) {
	result, err := SemanticNumber(42)
	if err != nil {
		t.Fatalf("SemanticNumber(42) error: %v", err)
	}
	if result != 42 {
		t.Errorf("SemanticNumber(42) = %f, want 42", result)
	}
}

func TestSemanticNumber_Float(t *testing.T) {
	result, err := SemanticNumber(3.14)
	if err != nil {
		t.Fatalf("SemanticNumber(3.14) error: %v", err)
	}
	if result != 3.14 {
		t.Errorf("SemanticNumber(3.14) = %f, want 3.14", result)
	}
}

func TestSemanticNumber_StringInt(t *testing.T) {
	result, err := SemanticNumber("42")
	if err != nil {
		t.Fatalf("SemanticNumber(\"42\") error: %v", err)
	}
	if result != 42 {
		t.Errorf("SemanticNumber(\"42\") = %f, want 42", result)
	}
}

func TestSemanticNumber_StringFloat(t *testing.T) {
	result, err := SemanticNumber("3.14")
	if err != nil {
		t.Fatalf("SemanticNumber(\"3.14\") error: %v", err)
	}
	if result != 3.14 {
		t.Errorf("SemanticNumber(\"3.14\") = %f, want 3.14", result)
	}
}

func TestSemanticNumber_StringNegative(t *testing.T) {
	result, err := SemanticNumber("-5")
	if err != nil {
		t.Fatalf("SemanticNumber(\"-5\") error: %v", err)
	}
	if result != -5 {
		t.Errorf("SemanticNumber(\"-5\") = %f, want -5", result)
	}
}

func TestSemanticNumber_StringAbc_Rejects(t *testing.T) {
	_, err := SemanticNumber("abc")
	if err == nil {
		t.Error("SemanticNumber(\"abc\") should return error")
	}
}

func TestSemanticNumber_EmptyString_Rejects(t *testing.T) {
	_, err := SemanticNumber("")
	if err == nil {
		t.Error("SemanticNumber(\"\") should return error")
	}
}

func TestSemanticNumber_Nil_Rejects(t *testing.T) {
	_, err := SemanticNumber(nil)
	if err == nil {
		t.Error("SemanticNumber(nil) should return error")
	}
}

func TestSemanticNumber_IntType(t *testing.T) {
	// JSON numbers come as float64, but test int path too
	result, err := SemanticNumber(int(10))
	if err != nil {
		t.Fatalf("SemanticNumber(int(10)) error: %v", err)
	}
	if result != 10 {
		t.Errorf("SemanticNumber(int(10)) = %f, want 10", result)
	}
}

func TestSemanticNumber_Int64Type(t *testing.T) {
	result, err := SemanticNumber(int64(100))
	if err != nil {
		t.Fatalf("SemanticNumber(int64(100)) error: %v", err)
	}
	if result != 100 {
		t.Errorf("SemanticNumber(int64(100)) = %f, want 100", result)
	}
}

// Test helper functions

func TestOptionalSemanticBool_Present(t *testing.T) {
	data := map[string]any{"flag": "true"}
	result, err := OptionalSemanticBool(data, "flag", false)
	if err != nil {
		t.Fatalf("OptionalSemanticBool error: %v", err)
	}
	if !result {
		t.Error("OptionalSemanticBool = false, want true")
	}
}

func TestOptionalSemanticBool_Missing(t *testing.T) {
	data := map[string]any{}
	result, err := OptionalSemanticBool(data, "flag", true)
	if err != nil {
		t.Fatalf("OptionalSemanticBool error: %v", err)
	}
	if !result {
		t.Error("OptionalSemanticBool = false, want true (default)")
	}
}

func TestOptionalSemanticBool_NativeBool(t *testing.T) {
	data := map[string]any{"flag": true}
	result, err := OptionalSemanticBool(data, "flag", false)
	if err != nil {
		t.Fatalf("OptionalSemanticBool error: %v", err)
	}
	if !result {
		t.Error("OptionalSemanticBool = false, want true")
	}
}

func TestOptionalSemanticBool_InvalidValue(t *testing.T) {
	data := map[string]any{"flag": "yes"}
	_, err := OptionalSemanticBool(data, "flag", false)
	if err == nil {
		t.Error("OptionalSemanticBool should return error for \"yes\"")
	}
}

func TestOptionalSemanticNumber_Present(t *testing.T) {
	data := map[string]any{"count": "42"}
	result, err := OptionalSemanticNumber(data, "count", 0)
	if err != nil {
		t.Fatalf("OptionalSemanticNumber error: %v", err)
	}
	if result != 42 {
		t.Errorf("OptionalSemanticNumber = %f, want 42", result)
	}
}

func TestOptionalSemanticNumber_Missing(t *testing.T) {
	data := map[string]any{}
	result, err := OptionalSemanticNumber(data, "count", 99)
	if err != nil {
		t.Fatalf("OptionalSemanticNumber error: %v", err)
	}
	if result != 99 {
		t.Errorf("OptionalSemanticNumber = %f, want 99 (default)", result)
	}
}

func TestOptionalSemanticNumber_NativeFloat(t *testing.T) {
	data := map[string]any{"count": float64(7)}
	result, err := OptionalSemanticNumber(data, "count", 0)
	if err != nil {
		t.Fatalf("OptionalSemanticNumber error: %v", err)
	}
	if result != 7 {
		t.Errorf("OptionalSemanticNumber = %f, want 7", result)
	}
}

func TestOptionalSemanticNumber_InvalidValue(t *testing.T) {
	data := map[string]any{"count": "abc"}
	_, err := OptionalSemanticNumber(data, "count", 0)
	if err == nil {
		t.Error("OptionalSemanticNumber should return error for \"abc\"")
	}
}
