package proxmox

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestClient(srv *httptest.Server) *ProxmoxClient {
	return &ProxmoxClient{
		baseURL:      srv.URL,
		node:         "pve01",
		tokenID:      "u@pam!t",
		tokenSecret:  "s",
		httpClient:   srv.Client(),
		pollInterval: 5 * time.Millisecond,
	}
}

func TestWaitTask_ReturnsOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"status":"stopped","exitstatus":"OK"}}`))
	}))
	defer srv.Close()

	exit, err := newTestClient(srv).WaitTask(context.Background(), "UPID:pve01:1")

	require.NoError(t, err)
	assert.Equal(t, "OK", exit)
}

func TestWaitTask_ReturnsFailureExitStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"status":"stopped","exitstatus":"device 0000:01:00.0 already in use"}}`))
	}))
	defer srv.Close()

	exit, err := newTestClient(srv).WaitTask(context.Background(), "UPID:pve01:1")

	require.NoError(t, err)
	assert.Equal(t, "device 0000:01:00.0 already in use", exit)
}

func TestWaitTask_PollsWhileRunning(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			_, _ = w.Write([]byte(`{"data":{"status":"running"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"status":"stopped","exitstatus":"OK"}}`))
	}))
	defer srv.Close()

	exit, err := newTestClient(srv).WaitTask(context.Background(), "UPID:pve01:1")

	require.NoError(t, err)
	assert.Equal(t, "OK", exit)
	assert.EqualValues(t, 3, calls.Load())
}

func TestWaitTask_RespectsContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"status":"running"}}`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err := newTestClient(srv).WaitTask(ctx, "UPID:pve01:1")

	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestWaitTask_EscapesUPIDInPath(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.EscapedPath()
		_, _ = w.Write([]byte(`{"data":{"status":"stopped","exitstatus":"OK"}}`))
	}))
	defer srv.Close()

	_, err := newTestClient(srv).WaitTask(context.Background(), "UPID:pve01:a/b")

	require.NoError(t, err)
	assert.Equal(t, "/api2/json/nodes/pve01/tasks/UPID:pve01:a%2Fb/status", got)
}
