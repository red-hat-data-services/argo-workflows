package gcs

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/api/googleapi"

	argoErrors "github.com/argoproj/argo-workflows/v3/errors"
)

func TestNewGCSClientWithCredential(t *testing.T) {
	t.Run("service account", func(t *testing.T) {
		client, err := newGCSClientWithCredential(`{"type":"service_account","project_id":"test-project","client_email":"test@test-project.iam.gserviceaccount.com"}`)
		require.NoError(t, err)
		require.NoError(t, client.Close())
	})
	for _, credentialType := range []string{"authorized_user", "external_account", "impersonated_service_account"} {
		t.Run(credentialType, func(t *testing.T) {
			client, err := newGCSClientWithCredential(fmt.Sprintf(`{"type":%q}`, credentialType))
			require.ErrorContains(t, err, `expected credential type "service_account"`)
			require.Nil(t, client)
		})
	}
	t.Run("invalid JSON", func(t *testing.T) {
		client, err := newGCSClientWithCredential("invalid")
		require.Error(t, err)
		require.Nil(t, client)
	})
}

type tlsHandshakeTimeoutError struct{}

func (tlsHandshakeTimeoutError) Temporary() bool { return true }
func (tlsHandshakeTimeoutError) Error() string   { return "net/http: TLS handshake timeout" }

func TestIsTransientGCSErr(t *testing.T) {
	for _, test := range []struct {
		err         error
		shouldretry bool
	}{
		{&googleapi.Error{Code: 0}, false},
		{argoErrors.New(argoErrors.CodeNotFound, "no results for key: foo/bar"), false},
		{&googleapi.Error{Code: 429}, true},
		{&googleapi.Error{Code: 504}, true},
		{&googleapi.Error{Code: 518}, true},
		{&googleapi.Error{Code: 599}, true},
		{&url.Error{Op: "blah", URL: "blah", Err: errors.New("connection refused")}, true},
		{&url.Error{Op: "blah", URL: "blah", Err: errors.New("compute: Received 504 `Gateway Timeout\n`")}, true},
		{&url.Error{Op: "blah", URL: "blah", Err: errors.New("http2: client connection lost")}, true},
		{io.ErrUnexpectedEOF, true},
		{&tlsHandshakeTimeoutError{}, true},
		{fmt.Errorf("Test unwrapping of a temporary error: %w", &googleapi.Error{Code: 500}), true},
		{fmt.Errorf("Test unwrapping of a non-retriable error: %w", &googleapi.Error{Code: 400}), false},
		{fmt.Errorf("writer close: Post \"https://storage.googleapis.com/upload/storage/v1/b/bucket/o?alt=json&name=test.json&uploadType=multipart\": compute: Received 504 `Gateway Timeout\n`"), true},
		{fmt.Errorf("http2: client connection lost"), true},
	} {
		got := isTransientGCSErr(test.err)
		if got != test.shouldretry {
			t.Errorf("%+v: got %v, want %v", test, got, test.shouldretry)
		}
	}
}
