package message

import (
	"bytes"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gpablo6/p3c/internal/history"
	"github.com/gpablo6/p3c/internal/workflow"
)

func TestCleanCommand_RejectsNegativeMaxCommits(t *testing.T) {
	svc := &workflow.RewriteService{
		OpenRepository: func(string) (*git.Repository, error) { return nil, nil },
		RunHistoryRewrite: func(_ *git.Repository, _ history.Config) (*history.Result, error) {
			t.Fatal("rewrite should not be called when validation fails")
			return nil, nil
		},
	}
	cmd := NewCommand(svc, func() (string, error) { return ".", nil }, func(s string) string { return s })

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"clean", "--max-commits", "-1"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--max-commits must be >= 0")
}
