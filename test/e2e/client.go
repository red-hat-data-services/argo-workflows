package e2e

import (
	"crypto/tls"
	"net/http"
)

var httpClient = &http.Client{
	Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

var http2Client = func() *http.Client {
	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	return &http.Client{
		Transport:     &http.Transport{Protocols: protocols},
		CheckRedirect: httpClient.CheckRedirect,
	}
}()
