package modules

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockModule_Init(t *testing.T) {
	m := &MockModule{}

	assert.True(t, m.Init())
	m.InitFunc = func() bool { return false }
	assert.False(t, m.Init())
}

func TestMockModule_Check(t *testing.T) {
	m := &MockModule{}

	assert.True(t, m.Check())
	m.CheckFunc = func() bool { return false }
	assert.False(t, m.Check())
}

func TestMockModule_Charts(t *testing.T) {
	m := &MockModule{}
	c := &Charts{}

	assert.Nil(t, m.Charts())
	m.ChartsFunc = func() *Charts { return c }
	assert.True(t, c == m.Charts())
}

func TestMockModule_GatherMetrics(t *testing.T) {
	m := &MockModule{}
	d := map[string]int64{
		"1": 1,
	}

	assert.Nil(t, m.GatherMetrics())
	m.GatherMetricsFunc = func() map[string]int64 { return d }
	assert.Equal(t, d, m.GatherMetrics())
}

func TestMockModule_Cleanup(t *testing.T) {
	m := &MockModule{}
	require.False(t, m.CleanupDone)

	m.Cleanup()
	assert.True(t, m.CleanupDone)
}