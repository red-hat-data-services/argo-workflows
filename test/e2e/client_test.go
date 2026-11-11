package e2e

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTP2ClientUsesH2CWithoutFollowingRedirects(t *testing.T) {
	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/redirected")
		w.WriteHeader(http.StatusFound)
	}))
	server.Config.Protocols = protocols
	server.Start()
	defer server.Close()

	response, err := http2Client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.ProtoMajor != 2 {
		t.Fatalf("expected HTTP/2, got %s", response.Proto)
	}
	if response.StatusCode != http.StatusFound {
		t.Fatalf("expected redirect response, got status %d", response.StatusCode)
	}
}
