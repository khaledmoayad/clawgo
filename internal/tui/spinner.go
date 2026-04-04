package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/spinner"
)

// SpinnerModel manages the loading spinner.
type SpinnerModel struct {
	spinner spinner.Model
	active  bool
	label   string
}

// NewSpinnerModel creates a spinner sub-model with a dot-style spinner.
func NewSpinnerModel() SpinnerModel {
	s := spinner.New(spinner.WithSpinner(spinner.Dot))
	return SpinnerModel{spinner: s, label: "Thinking"}
}

// Update processes spinner tick messages.
func (m SpinnerModel) Update(msg tea.Msg) (SpinnerModel, tea.Cmd) {
	if !m.active {
		return m, nil
	}
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

// View renders the spinner with its label when active.
func (m SpinnerModel) View() string {
	if !m.active {
		return ""
	}
	return DimStyle.Render(m.spinner.View() + " " + m.label + "...")
}

// Start activates the spinner with a label and returns the tick command.
func (m *SpinnerModel) Start(label string) tea.Cmd {
	m.active = true
	m.label = label
	return m.spinner.Tick
}

// Stop deactivates the spinner.
func (m *SpinnerModel) Stop() {
	m.active = false
}

// IsActive returns whether the spinner is currently spinning.
func (m SpinnerModel) IsActive() bool { return m.active }
