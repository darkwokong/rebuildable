package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jayeshsarvaiya/rebuildable/internal/testutil"
)

func TestAuditCommand(t *testing.T) {
	root := testutil.CopyFixture(t, "good-go")
	testutil.InitGitRepo(t, root, 3)

	command := newRootCommand()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs([]string{"audit", root})
	if err := command.Execute(); err != nil {
		t.Fatalf("execute audit: %v", err)
	}
	if !strings.Contains(output.String(), "Score 3/8") {
		t.Fatalf("audit output does not contain score:\n%s", output.String())
	}
}
