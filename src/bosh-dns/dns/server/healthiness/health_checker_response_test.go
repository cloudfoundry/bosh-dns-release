package healthiness_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"bosh-dns/dns/server/healthiness"
	"bosh-dns/dns/server/healthiness/healthinessfakes"

	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
)

type responseBody struct {
	io.Reader
	closed bool
	count  int
}

func (b *responseBody) Read(p []byte) (int, error) {
	n, e := b.Reader.Read(p)
	b.count += n
	return n, e
}
func (b *responseBody) Close() error { b.closed = true; return nil }

func TestHealthResponseSizeAndClosure(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusInternalServerError} {
		body := &responseBody{Reader: strings.NewReader(strings.Repeat(" ", 2<<20))}
		client := &healthinessfakes.FakeHTTPClientGetter{}
		client.GetReturns(&http.Response{StatusCode: status, Body: body}, nil)
		checker := healthiness.NewHealthChecker(client, 8081, &loggerfakes.FakeLogger{})
		if got := checker.GetStatus("127.0.0.1"); got.State != healthiness.StateUnknown {
			t.Fatalf("unexpected state: %v", got.State)
		}
		if !body.closed {
			t.Fatal("response body was not closed")
		}
		if body.count > 1<<20+1 {
			t.Fatalf("read too much response data: %d", body.count)
		}
	}
}
