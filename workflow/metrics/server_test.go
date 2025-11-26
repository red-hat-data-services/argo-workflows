package metrics

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDisableMetricsServer(t *testing.T) {
	config := ServerConfig{
		Enabled: false,
		Path:    DefaultMetricsServerPath,
		Port:    DefaultMetricsServerPort,
	}
	m := New(config, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m.RunServer(ctx, false)
	time.Sleep(1 * time.Second) // to confirm that the server doesn't start, even if we wait
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d%s", DefaultMetricsServerPort, DefaultMetricsServerPath))
	if resp != nil {
		defer resp.Body.Close()
	}

	assert.Error(t, err)

	// Check for connection refused error message (different on Windows vs Unix)
	errMsg := strings.ToLower(err.Error())
	if runtime.GOOS == "windows" {
		assert.True(t, strings.Contains(errMsg, "refused") || strings.Contains(errMsg, "connection") || strings.Contains(errMsg, "target machine"), "Expected connection refused error, got: %s", err.Error())
	} else {
		assert.Contains(t, errMsg, "connection refused", "Expected connection refused error, got: %s", err.Error())
	}
}

func TestMetricsServer(t *testing.T) {
	config := ServerConfig{
		Enabled: true,
		Path:    DefaultMetricsServerPath,
		Port:    DefaultMetricsServerPort,
	}
	m := New(config, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m.RunServer(ctx, false)
	time.Sleep(1 * time.Second)
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d%s", DefaultMetricsServerPort, DefaultMetricsServerPath))
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	bodyString := string(bodyBytes)
	assert.NotEmpty(t, bodyString)
}

func TestDummyMetricsServer(t *testing.T) {
	config := ServerConfig{
		Enabled: true,
		Path:    DefaultMetricsServerPath,
		Port:    DefaultMetricsServerPort,
	}
	m := New(config, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m.RunServer(ctx, true)
	time.Sleep(1 * time.Second)
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d%s", DefaultMetricsServerPort, DefaultMetricsServerPath))
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	bodyString := string(bodyBytes)

	assert.Empty(t, bodyString) // expect the dummy metrics server to provide no metrics responses
}
