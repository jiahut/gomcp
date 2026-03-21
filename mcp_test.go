package main

import "testing"

func TestToolMode(t *testing.T) {
	srv := &MCPServer{}

	if got := srv.toolMode("over"); got != toolExecutionStateless {
		t.Fatalf("expected over to be stateless, got %v", got)
	}

	statefulTools := []string{"goto", "google", "duckduckgo", "markdown", "links"}
	for _, name := range statefulTools {
		if got := srv.toolMode(name); got != toolExecutionStateful {
			t.Fatalf("expected %s to be stateful, got %v", name, got)
		}
	}
}
