package handlers_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bosh-dns/dns/server/handlers"
	"bosh-dns/dns/server/handlers/handlersfakes"
	"bosh-dns/dns/server/internal/internalfakes"
	"bosh-dns/dns/server/records/dnsresolver/dnsresolverfakes"

	"code.cloudfoundry.org/clock"
	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	"github.com/miekg/dns"
)

type countedResponseBody struct {
	io.Reader
	count  int
	closed bool
}

func (b *countedResponseBody) Read(p []byte) (int, error) {
	n, e := b.Reader.Read(p)
	b.count += n
	return n, e
}
func (b *countedResponseBody) Close() error { b.closed = true; return nil }

func TestHTTPDNSResponseLimit(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusInternalServerError} {
		body := &countedResponseBody{Reader: strings.NewReader(strings.Repeat(" ", 2<<20))}
		client := &handlersfakes.FakeHTTPClient{}
		client.GetReturns(&http.Response{StatusCode: status, Body: body}, nil)
		handler := handlers.NewHTTPJSONHandler("http://example.test", client, &loggerfakes.FakeLogger{}, &dnsresolverfakes.FakeResponseTruncater{})
		req := new(dns.Msg)
		req.SetQuestion("example.test.", dns.TypeA)
		writer := &internalfakes.FakeResponseWriter{}
		handler.ServeDNS(writer, req)
		if writer.WriteMsgArgsForCall(0).Rcode != dns.RcodeServerFailure {
			t.Fatal("oversized/error response accepted")
		}
		if !body.closed {
			t.Fatal("response body not closed")
		}
		if body.count > 1<<20+1 {
			t.Fatalf("read %d response bytes", body.count)
		}
		if status != http.StatusOK && body.count != 0 {
			t.Fatal("read error response body")
		}
	}
}

func TestHTTPDNSFactoryRejectsUntrustedTLS(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.WriteString(w, `{"Answer":[]}`); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	factory := handlers.NewFactory(nil, clock.NewClock(), 1, &loggerfakes.FakeLogger{}, &dnsresolverfakes.FakeResponseTruncater{})
	handler := factory.CreateHTTPJSONHandler(server.URL, false)
	req := new(dns.Msg)
	req.SetQuestion("example.test.", dns.TypeA)
	writer := &internalfakes.FakeResponseWriter{}
	handler.ServeDNS(writer, req)
	if writer.WriteMsgArgsForCall(0).Rcode != dns.RcodeServerFailure {
		t.Fatal("untrusted TLS certificate accepted")
	}
}
