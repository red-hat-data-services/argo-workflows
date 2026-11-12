//go:build !windows

package telemetry

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	promgo "github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// testScopeName is the name that the metrics running under test will have
const testScopeName string = "argo-workflows-test"

func TestPrometheusMetricNames(t *testing.T) {
	registry := promgo.NewRegistry()
	previousRegisterer := promgo.DefaultRegisterer
	promgo.DefaultRegisterer = registry
	t.Cleanup(func() { promgo.DefaultRegisterer = previousRegisterer })

	exporter, err := (&Config{}).prometheusMetricsExporter("argo")
	require.NoError(t, err)
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter))
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })
	counter, err := provider.Meter(testScopeName).Int64Counter("test.duration", metric.WithUnit("s"))
	require.NoError(t, err)
	counter.Add(context.Background(), 3)

	families, err := registry.Gather()
	require.NoError(t, err)
	require.Len(t, families, 1)
	assert.Equal(t, "argo_test_duration", families[0].GetName())
	assert.InDelta(t, 3, families[0].Metric[0].GetCounter().GetValue(), 0.001)
}

func TestDisablePrometheusServer(t *testing.T) {
	config := Config{
		Enabled: false,
		Path:    DefaultPrometheusServerPath,
		Port:    DefaultPrometheusServerPort,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m, err := NewMetrics(ctx, testScopeName, testScopeName, &config)
	require.NoError(t, err)
	m.RunPrometheusServer(ctx, false)
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d%s", DefaultPrometheusServerPort, DefaultPrometheusServerPath))
	if resp != nil {
		defer resp.Body.Close()
	}

	require.ErrorContains(t, err, "connection refused") // expect that the metrics server not to start
}

func TestPrometheusServer(t *testing.T) {
	var wg sync.WaitGroup
	config := Config{
		Enabled: true,
		Path:    DefaultPrometheusServerPath,
		Port:    DefaultPrometheusServerPort,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m, err := NewMetrics(ctx, testScopeName, testScopeName, &config)
	require.NoError(t, err)
	wg.Add(1)
	go func() {
		m.RunPrometheusServer(ctx, false)
		wg.Done()
	}()
	time.Sleep(1 * time.Second)
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d%s", DefaultPrometheusServerPort, DefaultPrometheusServerPath))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	bodyString := string(bodyBytes)
	assert.NotEmpty(t, bodyString)

	cancel() // cancel and wait for server shutdown to prevent port conflicts with subsequent tests
	wg.Wait()
}

func TestDummyPrometheusServer(t *testing.T) {
	var wg sync.WaitGroup
	config := Config{
		Enabled: true,
		Path:    DefaultPrometheusServerPath,
		Port:    DefaultPrometheusServerPort,
		Secure:  false,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m, err := NewMetrics(ctx, testScopeName, testScopeName, &config)
	require.NoError(t, err)
	wg.Add(1)
	go func() {
		m.RunPrometheusServer(ctx, true)
		wg.Done()
	}()
	time.Sleep(1 * time.Second)
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d%s", DefaultPrometheusServerPort, DefaultPrometheusServerPath))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	bodyString := string(bodyBytes)

	assert.Empty(t, bodyString) // expect the dummy metrics server to provide no metrics responses

	cancel() // cancel and wait for server shutdown to prevent port conflicts with subsequent tests
	wg.Wait()
}
