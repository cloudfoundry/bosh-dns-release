package handlers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"

	"net/url"
	"strconv"

	"bosh-dns/dns/server/records/dnsresolver"

	"github.com/cloudfoundry/bosh-utils/httpclient"
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"
	"net/http"
)

//go:generate counterfeiter . HTTPClient

type HTTPClient interface {
	Get(endpoint string) (*http.Response, error)
}

type HTTPJSONHandler struct {
	Address string
	Client  HTTPClient
	Logger  logger.Logger
	LogTag  string
}

type Answer struct {
	Name   string `json:"name"`
	RRType uint16 `json:"type"`
	TTL    uint32 `json:"TTL"`
	Data   string `json:"data"`
}

type httpDNSMessage struct {
	Truncated bool     `json:"TC"`
	Answer    []Answer `json:"Answer"`
}

func NewHTTPJSONHandler(address string, logger logger.Logger) HTTPJSONHandler {
	return HTTPJSONHandler{
		Address: address,
		Client: httpclient.NewHTTPClient(
			httpclient.DefaultClient,
			logger,
		),
		Logger: logger,
		LogTag: "HTTPJSONHandler",
	}
}

func (h HTTPJSONHandler) ServeDNS(responseWriter dns.ResponseWriter, request *dns.Msg) {
	responseMsg := h.buildResponse(request)

	dnsresolver.TruncateIfNeeded(responseWriter, responseMsg)

	if err := responseWriter.WriteMsg(responseMsg); err != nil {
		h.Logger.Error(h.LogTag, err.Error())
	}
}

func (h HTTPJSONHandler) buildResponse(request *dns.Msg) *dns.Msg {
	responseMsg := new(dns.Msg)
	responseMsg.Authoritative = true
	responseMsg.RecursionAvailable = true
	responseMsg.SetReply(request)

	if len(request.Question) == 0 {
		return responseMsg
	}

	question := request.Question[0]

	queryParams := url.Values{
		"type": []string{strconv.Itoa(int(question.Qtype))},
		"name": []string{question.Name},
	}.Encode()

	url := fmt.Sprintf("%s/?%s", h.Address, queryParams)
	httpResponse, err := h.Client.Get(url)

	if err != nil {
		h.Logger.Error(h.LogTag, "error connecting to '%s': %v", h.Address, err)
		responseMsg.SetRcode(request, dns.RcodeServerFailure)
		return responseMsg
	}

	defer func() {
		ioutil.ReadAll(httpResponse.Body)
		httpResponse.Body.Close()
	}()

	if httpResponse.StatusCode != 200 {
		h.Logger.Error(h.LogTag, "non successful response from server '%s': %v", h.Address, httpResponse)
		responseMsg.SetRcode(request, dns.RcodeServerFailure)
		return responseMsg
	}

	httpDNSMessage := &httpDNSMessage{}
	bytes, err := ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		h.Logger.Error(h.LogTag, "failed to read response message '%s': %v", string(bytes), err)
		responseMsg.SetRcode(request, dns.RcodeServerFailure)
		return responseMsg
	}

	err = json.Unmarshal(bytes, httpDNSMessage)
	if err != nil {
		h.Logger.Error(h.LogTag, "failed to unmarshal response message '%s': %v", string(bytes), err)
		responseMsg.SetRcode(request, dns.RcodeServerFailure)
		return responseMsg
	}

	responseMsg.Truncated = httpDNSMessage.Truncated
	for _, answer := range httpDNSMessage.Answer {
		responseMsg.Answer = append(responseMsg.Answer, &dns.A{
			Hdr: dns.RR_Header{
				Name:   question.Name,
				Rrtype: answer.RRType,
				Class:  dns.ClassINET,
				Ttl:    answer.TTL,
			},
			A: net.ParseIP(answer.Data),
		})
	}

	responseMsg.SetRcode(request, dns.RcodeSuccess)
	return responseMsg
}
