package testutil

import (
	"testing"
)

type Unregisterer interface {
	UnregisterAll()
}

func NewTestMetrics(t *testing.T, m Unregisterer) {
	t.Helper()
	t.Cleanup(m.UnregisterAll)
}
