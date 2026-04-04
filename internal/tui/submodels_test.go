package tui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInputModel_Value(t *testing.T) {
	m := NewInputModel()
	// Initial value should be empty
	assert.Equal(t, "", m.Value())
}

func TestInputModel_Reset(t *testing.T) {
	m := NewInputModel()
	// Set some value via textarea then reset
	m.textarea.SetValue("hello world")
	require.Equal(t, "hello world", m.Value())

	m.Reset()
	assert.Equal(t, "", m.Value())
}

func TestOutputModel_AddMessage(t *testing.T) {
	m := NewOutputModel()
	msg := DisplayMessage{Role: "user", Content: "hello"}
	m.AddMessage(msg)

	require.Len(t, m.Messages(), 1)
	assert.Equal(t, "user", m.Messages()[0].Role)
	assert.Equal(t, "hello", m.Messages()[0].Content)
}

func TestOutputModel_AppendStreaming(t *testing.T) {
	m := NewOutputModel()
	m.StartStreaming()
	m.AppendStreaming("hello ")
	m.AppendStreaming("world")

	assert.Equal(t, "hello world", m.streamingText.String())
}

func TestOutputModel_FinishStreaming(t *testing.T) {
	m := NewOutputModel()
	m.StartStreaming()
	m.AppendStreaming("complete response")
	m.FinishStreaming()

	require.Len(t, m.Messages(), 1)
	assert.Equal(t, "assistant", m.Messages()[0].Role)
	assert.Equal(t, "complete response", m.Messages()[0].Content)
	assert.False(t, m.isStreaming)
}

func TestOutputModel_View(t *testing.T) {
	m := NewOutputModel()
	m.AddMessage(DisplayMessage{Role: "user", Content: "hi"})
	m.AddMessage(DisplayMessage{Role: "assistant", Content: "hello"})

	view := m.View()
	// View should contain role labels for both messages
	assert.True(t, strings.Contains(view, "hi"), "view should contain user message content")
	assert.True(t, strings.Contains(view, "hello"), "view should contain assistant message content")
}

func TestOutputModel_ViewStreaming(t *testing.T) {
	m := NewOutputModel()
	m.StartStreaming()
	m.AppendStreaming("streaming text")

	view := m.View()
	assert.True(t, strings.Contains(view, "streaming text"), "view should contain streaming text")
}

func TestOutputModel_Clear(t *testing.T) {
	m := NewOutputModel()
	m.AddMessage(DisplayMessage{Role: "user", Content: "hi"})
	m.Clear()

	assert.Len(t, m.Messages(), 0)
}

func TestSpinnerModel_ViewWhenInactive(t *testing.T) {
	m := NewSpinnerModel()
	// Spinner should be inactive by default
	assert.Equal(t, "", m.View())
	assert.False(t, m.IsActive())
}

func TestSpinnerModel_StartAndStop(t *testing.T) {
	m := NewSpinnerModel()
	m.Start("Thinking")
	assert.True(t, m.IsActive())

	m.Stop()
	assert.False(t, m.IsActive())
	assert.Equal(t, "", m.View())
}

func TestSpinnerModel_ViewWhenActive(t *testing.T) {
	m := NewSpinnerModel()
	m.Start("Loading")
	// When active, View should return non-empty string containing the label
	view := m.View()
	assert.NotEmpty(t, view)
	assert.True(t, strings.Contains(view, "Loading"), "spinner view should contain the label")
}
