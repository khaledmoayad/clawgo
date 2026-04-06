package swarm

import (
	"testing"
)

func TestFormatAgentID(t *testing.T) {
	tests := []struct {
		name     string
		agent    string
		team     string
		expected string
	}{
		{"basic", "researcher", "my-project", "researcher@my-project"},
		{"team-lead", "team-lead", "alpha", "team-lead@alpha"},
		{"empty agent", "", "team", "@team"},
		{"empty team", "agent", "", "agent@"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatAgentID(tt.agent, tt.team)
			if got != tt.expected {
				t.Errorf("FormatAgentID(%q, %q) = %q, want %q", tt.agent, tt.team, got, tt.expected)
			}
		})
	}
}

func TestParseAgentID(t *testing.T) {
	tests := []struct {
		name      string
		agentID   string
		wantAgent string
		wantTeam  string
		wantOk    bool
	}{
		{"valid", "researcher@my-project", "researcher", "my-project", true},
		{"team-lead", "team-lead@alpha", "team-lead", "alpha", true},
		{"no separator", "agent-abc123", "", "", false},
		{"empty agent", "@team", "", "team", true},
		{"empty team", "agent@", "agent", "", true},
		{"multiple @", "a@b@c", "a", "b@c", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent, team, ok := ParseAgentID(tt.agentID)
			if ok != tt.wantOk {
				t.Errorf("ParseAgentID(%q) ok = %v, want %v", tt.agentID, ok, tt.wantOk)
			}
			if agent != tt.wantAgent {
				t.Errorf("ParseAgentID(%q) agent = %q, want %q", tt.agentID, agent, tt.wantAgent)
			}
			if team != tt.wantTeam {
				t.Errorf("ParseAgentID(%q) team = %q, want %q", tt.agentID, team, tt.wantTeam)
			}
		})
	}
}

func TestFormatParseRoundTrip(t *testing.T) {
	names := [][2]string{
		{"researcher", "my-project"},
		{"tester", "alpha-team"},
		{"team-lead", "build-system"},
	}
	for _, pair := range names {
		id := FormatAgentID(pair[0], pair[1])
		agent, team, ok := ParseAgentID(id)
		if !ok {
			t.Errorf("ParseAgentID(%q) returned not ok", id)
			continue
		}
		if agent != pair[0] || team != pair[1] {
			t.Errorf("round trip failed: FormatAgentID(%q, %q) -> ParseAgentID -> (%q, %q)",
				pair[0], pair[1], agent, team)
		}
	}
}

func TestSanitizeAgentName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"researcher", "researcher"},
		{"re@searcher", "researcher"},
		{"@leader@", "leader"},
		{"no-at-signs", "no-at-signs"},
		{"@@", ""},
	}

	for _, tt := range tests {
		got := SanitizeAgentName(tt.input)
		if got != tt.expected {
			t.Errorf("SanitizeAgentName(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestFormatRequestID(t *testing.T) {
	id := FormatRequestID("shutdown", "researcher@my-project", 1702500000000)
	expected := "shutdown-1702500000000@researcher@my-project"
	if id != expected {
		t.Errorf("FormatRequestID = %q, want %q", id, expected)
	}
}

func TestParseAgentRequestID(t *testing.T) {
	tests := []struct {
		name        string
		requestID   string
		wantType    string
		wantTS      int64
		wantAgentID string
		wantOk      bool
	}{
		{
			"shutdown request",
			"shutdown-1702500000000@researcher@my-project",
			"shutdown", 1702500000000, "researcher@my-project", true,
		},
		{
			"plan approval",
			"plan-approval-1702500001000@tester@alpha",
			"plan-approval", 1702500001000, "tester@alpha", true,
		},
		{"no at sign", "shutdown-123", "", 0, "", false},
		{"no dash in prefix", "shutdown@agent@team", "", 0, "", false},
		{"non-numeric timestamp", "shutdown-abc@agent@team", "", 0, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqType, ts, agentID, ok := ParseAgentRequestID(tt.requestID)
			if ok != tt.wantOk {
				t.Errorf("ParseAgentRequestID(%q) ok = %v, want %v", tt.requestID, ok, tt.wantOk)
				return
			}
			if !ok {
				return
			}
			if reqType != tt.wantType {
				t.Errorf("ParseAgentRequestID(%q) type = %q, want %q", tt.requestID, reqType, tt.wantType)
			}
			if ts != tt.wantTS {
				t.Errorf("ParseAgentRequestID(%q) ts = %d, want %d", tt.requestID, ts, tt.wantTS)
			}
			if agentID != tt.wantAgentID {
				t.Errorf("ParseAgentRequestID(%q) agentID = %q, want %q", tt.requestID, agentID, tt.wantAgentID)
			}
		})
	}
}

func TestParseAgentRequestIDRoundTrip(t *testing.T) {
	agentID := "researcher@my-project"
	ts := int64(1702500000000)
	requestID := FormatRequestID("shutdown", agentID, ts)
	reqType, gotTS, gotAgentID, ok := ParseAgentRequestID(requestID)
	if !ok {
		t.Fatalf("ParseAgentRequestID(%q) returned not ok", requestID)
	}
	if reqType != "shutdown" {
		t.Errorf("type = %q, want %q", reqType, "shutdown")
	}
	if gotTS != ts {
		t.Errorf("ts = %d, want %d", gotTS, ts)
	}
	if gotAgentID != agentID {
		t.Errorf("agentID = %q, want %q", gotAgentID, agentID)
	}
}
