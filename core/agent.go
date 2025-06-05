package core

// Agent is an interface that all Aquatone agents must implement.
type Agent interface {
	// ID returns a unique identifier for the agent.
	ID() string
	// Register subscribes the agent to relevant events on the session's event bus.
	Register(session *Session) error
}
