package controller

import "testing"

// The Backstage backend derives the same token (selfservice/agent.ts); both
// sides pin this vector so they cannot drift apart.
func TestAgentTokenVector(t *testing.T) {
	const want = "4db4b6dffe33b18525ca87a81968e519a1749b3658c5f44ee1cb3c2e63c954e9"
	if got := AgentToken("koptan-test-key", "team", "shop"); got != want {
		t.Errorf("AgentToken = %s, want %s", got, want)
	}
	if AgentToken("koptan-test-key", "team", "shop2") == want {
		t.Error("tokens must differ per SelfService")
	}
}
