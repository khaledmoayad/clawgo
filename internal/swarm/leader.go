package swarm

import (
	"github.com/khaledmoayad/clawgo/internal/api"
)

// Leader wraps a Manager with additional context for the leader agent.
// The leader is responsible for reading task notifications and injecting
// them as user-role messages into the coordinator's conversation loop.
type Leader struct {
	Manager *Manager
}

// NewLeader creates a Leader wrapping the given Manager.
func NewLeader(manager *Manager) *Leader {
	return &Leader{Manager: manager}
}

// ProcessNotification converts a TaskNotification into a user-role Message
// containing the XML representation. The coordinator's query loop injects
// these as user messages so Claude sees them naturally in the conversation.
func (l *Leader) ProcessNotification(notif TaskNotification) api.Message {
	return api.UserMessage(notif.ToXML())
}
