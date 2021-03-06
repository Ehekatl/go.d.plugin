package tengine

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/netdata/go-orchestrator/module"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testStatusData, _ = ioutil.ReadFile("testdata/status.txt")
)

func TestTengine_Cleanup(t *testing.T) { New().Cleanup() }

func TestNew(t *testing.T) {
	job := New()

	assert.Implements(t, (*module.Module)(nil), job)
	assert.Equal(t, defaultURL, job.UserURL)
	assert.Equal(t, defaultHTTPTimeout, job.Timeout.Duration)
}

func TestTengine_Init(t *testing.T) {
	job := New()

	require.True(t, job.Init())
	assert.NotNil(t, job.apiClient)
}

func TestTengine_Check(t *testing.T) {
	ts := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write(testStatusData)
			}))
	defer ts.Close()

	job := New()
	job.UserURL = ts.URL
	require.True(t, job.Init())
	assert.True(t, job.Check())
}

func TestTengine_CheckNG(t *testing.T) {
	job := New()

	job.UserURL = "http://127.0.0.1:38001/us"
	require.True(t, job.Init())
	assert.False(t, job.Check())
}

func TestTengine_Charts(t *testing.T) { assert.NotNil(t, New().Charts()) }

func TestTengine_Collect(t *testing.T) {
	ts := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write(testStatusData)
			}))
	defer ts.Close()

	job := New()
	job.UserURL = ts.URL
	require.True(t, job.Init())
	require.True(t, job.Check())

	expected := map[string]int64{
		"bytes_in":                 5944,
		"bytes_out":                20483,
		"conn_total":               64,
		"http_200":                 65,
		"http_206":                 0,
		"http_2xx":                 65,
		"http_302":                 0,
		"http_304":                 0,
		"http_3xx":                 0,
		"http_403":                 0,
		"http_404":                 0,
		"http_416":                 0,
		"http_499":                 0,
		"http_4xx":                 0,
		"http_500":                 0,
		"http_502":                 0,
		"http_503":                 0,
		"http_504":                 0,
		"http_508":                 0,
		"http_5xx":                 0,
		"http_other_detail_status": 0,
		"http_other_status":        0,
		"http_ups_4xx":             0,
		"http_ups_5xx":             0,
		"req_total":                65,
		"rt":                       0,
		"ups_req":                  0,
		"ups_rt":                   0,
		"ups_tries":                0,
	}

	assert.Equal(t, expected, job.Collect())
}

func TestTengine_InvalidData(t *testing.T) {
	ts := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("hello and goodbye"))
			}))
	defer ts.Close()

	job := New()
	job.UserURL = ts.URL
	require.True(t, job.Init())
	assert.False(t, job.Check())
}

func TestTengine_404(t *testing.T) {
	ts := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			}))
	defer ts.Close()

	job := New()
	job.UserURL = ts.URL
	require.True(t, job.Init())
	assert.False(t, job.Check())
}
