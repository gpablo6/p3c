package identity

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gpablo6/p3c/internal/history"
)

func TestBuildIdentity_AllowsCombinedAndSplitForms(t *testing.T) {
	combined, err := buildIdentity("gpablo6 <gpablo6@outlook.com>", "", "", "from")
	require.NoError(t, err)
	assert.Equal(t, history.Identity{Name: "gpablo6", Email: "gpablo6@outlook.com"}, combined)

	split, err := buildIdentity("", "gpablo6", "gpablo6@outlook.com", "to")
	require.NoError(t, err)
	assert.Equal(t, history.Identity{Name: "gpablo6", Email: "gpablo6@outlook.com"}, split)
}

func TestBuildIdentity_RejectsMixedOrIncompleteInput(t *testing.T) {
	_, err := buildIdentity("gpablo6 <gpablo6@outlook.com>", "gpablo6", "", "from")
	require.Error(t, err)

	_, err = buildIdentity("", "gpablo6", "", "from")
	require.Error(t, err)
}
