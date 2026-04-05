package query

import (
	"testing"
	"time"
)

func TestBudgetTracker_NoBudget(t *testing.T) {
	tracker := NewBudgetTracker()

	// budget <= 0 should always return stop with no event
	decision := CheckTokenBudget(tracker, "", 0, 1000)
	if decision.Action() != "stop" {
		t.Errorf("expected stop, got %s", decision.Action())
	}
	stop := decision.(StopDecision)
	if stop.CompletionEvent != nil {
		t.Error("expected nil CompletionEvent for no budget")
	}
}

func TestBudgetTracker_AgentIDAlwaysStops(t *testing.T) {
	tracker := NewBudgetTracker()

	// Any non-empty agentID should always return stop
	decision := CheckTokenBudget(tracker, "agent-1", 10000, 1000)
	if decision.Action() != "stop" {
		t.Errorf("expected stop for agent ID, got %s", decision.Action())
	}
	stop := decision.(StopDecision)
	if stop.CompletionEvent != nil {
		t.Error("expected nil CompletionEvent for agent")
	}
}

func TestBudgetTracker_UnderThresholdContinues(t *testing.T) {
	tracker := NewBudgetTracker()

	// 5000 / 10000 = 50%, well under 90%
	decision := CheckTokenBudget(tracker, "", 10000, 5000)
	if decision.Action() != "continue" {
		t.Fatalf("expected continue, got %s", decision.Action())
	}

	cont := decision.(ContinueDecision)
	if cont.Pct != 50 {
		t.Errorf("expected pct=50, got %d", cont.Pct)
	}
	if cont.TurnTokens != 5000 {
		t.Errorf("expected turnTokens=5000, got %d", cont.TurnTokens)
	}
	if cont.Budget != 10000 {
		t.Errorf("expected budget=10000, got %d", cont.Budget)
	}
	if cont.ContinuationCount != 1 {
		t.Errorf("expected continuationCount=1, got %d", cont.ContinuationCount)
	}
	if cont.NudgeMessage == "" {
		t.Error("expected non-empty nudge message")
	}
}

func TestBudgetTracker_AtThresholdStops(t *testing.T) {
	tracker := NewBudgetTracker()

	// First call under threshold to set continuation count > 0
	decision := CheckTokenBudget(tracker, "", 10000, 5000)
	if decision.Action() != "continue" {
		t.Fatalf("expected first call to continue, got %s", decision.Action())
	}

	// Second call at 90% threshold: 9000/10000 = 90%
	decision = CheckTokenBudget(tracker, "", 10000, 9000)
	if decision.Action() != "stop" {
		t.Fatalf("expected stop at 90%%, got %s", decision.Action())
	}

	stop := decision.(StopDecision)
	if stop.CompletionEvent == nil {
		t.Fatal("expected CompletionEvent at threshold with prior continuations")
	}
	if stop.CompletionEvent.Pct != 90 {
		t.Errorf("expected pct=90, got %d", stop.CompletionEvent.Pct)
	}
	if stop.CompletionEvent.ContinuationCount != 1 {
		t.Errorf("expected continuationCount=1, got %d", stop.CompletionEvent.ContinuationCount)
	}
}

func TestBudgetTracker_DiminishingReturns(t *testing.T) {
	tracker := NewBudgetTracker()
	tracker.StartedAt = time.Now().Add(-5 * time.Second)

	budget := 100000

	// Simulate 4 checks with small deltas to trigger diminishing returns
	// Check 1: 10000 tokens (first call, delta from 0 = 10000 > 500)
	d := CheckTokenBudget(tracker, "", budget, 10000)
	if d.Action() != "continue" {
		t.Fatalf("check 1: expected continue, got %s", d.Action())
	}

	// Check 2: 10100 tokens (delta = 100 < 500, but count only 1)
	d = CheckTokenBudget(tracker, "", budget, 10100)
	if d.Action() != "continue" {
		t.Fatalf("check 2: expected continue, got %s", d.Action())
	}

	// Check 3: 10200 tokens (delta = 100 < 500, count=2)
	d = CheckTokenBudget(tracker, "", budget, 10200)
	if d.Action() != "continue" {
		t.Fatalf("check 3: expected continue, got %s", d.Action())
	}

	// Check 4: 10300 tokens (delta = 100 < 500, count=3, lastDelta=100 < 500 -> diminishing!)
	d = CheckTokenBudget(tracker, "", budget, 10300)
	if d.Action() != "stop" {
		t.Fatalf("check 4: expected stop (diminishing), got %s", d.Action())
	}

	stop := d.(StopDecision)
	if stop.CompletionEvent == nil {
		t.Fatal("expected CompletionEvent for diminishing returns")
	}
	if !stop.CompletionEvent.DiminishingReturns {
		t.Error("expected DiminishingReturns=true")
	}
	if stop.CompletionEvent.DurationMs <= 0 {
		t.Errorf("expected positive duration, got %d", stop.CompletionEvent.DurationMs)
	}
}

func TestBudgetTracker_FirstCallAtThresholdNoEvent(t *testing.T) {
	tracker := NewBudgetTracker()

	// First call already at 95% with no prior continuations
	decision := CheckTokenBudget(tracker, "", 10000, 9500)
	if decision.Action() != "stop" {
		t.Fatalf("expected stop, got %s", decision.Action())
	}

	stop := decision.(StopDecision)
	if stop.CompletionEvent != nil {
		t.Error("expected nil CompletionEvent for first call at threshold")
	}
}

func TestGetBudgetContinuationMessage(t *testing.T) {
	msg := GetBudgetContinuationMessage(50, 5000, 10000)
	if msg == "" {
		t.Error("expected non-empty message")
	}

	// Check it contains expected parts
	expected := "Stopped at 50% of token target"
	if len(msg) < len(expected) {
		t.Errorf("message too short: %s", msg)
	}

	// Should contain formatted numbers with commas
	msg2 := GetBudgetContinuationMessage(75, 75000, 100000)
	if msg2 == "" {
		t.Error("expected non-empty message for larger numbers")
	}
}

func TestBudgetTracker_NegativeBudget(t *testing.T) {
	tracker := NewBudgetTracker()

	decision := CheckTokenBudget(tracker, "", -100, 5000)
	if decision.Action() != "stop" {
		t.Errorf("expected stop for negative budget, got %s", decision.Action())
	}
}
