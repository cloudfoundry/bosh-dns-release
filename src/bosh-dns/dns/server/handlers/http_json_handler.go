package handlers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"

	"net/url"
	"strconv"

	"bosh-dns/dns/server/records/dnsresolver"

	"net/http"

	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"
)

//go:generate counterfeiter . HTTPClient

type HTTPClient interface {
	Get(endpoint string) (*http.Response, error)
}

type HTTPJSONHandler struct {
	address string
	client  HTTPClient
	logger  logger.Logger
	logTag  string
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

func NewHTTPJSONHandler(address string, httpClient HTTPClient, logger logger.Logger) HTTPJSONHandler {
	return HTTPJSONHandler{
		address: address,
		client:  httpClient,
		logger:  logger,
		logTag:  "HTTPJSONHandler",
	}
}

func (h HTTPJSONHandler) ServeDNS(responseWriter dns.ResponseWriter, request *dns.Msg) {
	responseMsg := h.buildResponse(request)

	dnsresolver.TruncateIfNeeded(responseWriter, responseMsg)

	if err := responseWriter.WriteMsg(responseMsg); err != nil {
		h.logger.Error(h.logTag, err.Error())
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

	url := fmt.Sprintf("%s/?%s", h.address, queryParams)
	httpResponse, err := h.client.Get(url)

	if err != nil {
		h.logger.Error(h.logTag, "error connecting to '%s': %v", h.address, err)
		responseMsg.SetRcode(request, dns.RcodeServerFailure)
		return responseMsg
	}

	defer func() {
		ioutil.ReadAll(httpResponse.Body)
		httpResponse.Body.Close()
	}()

	if httpResponse.StatusCode != 200 {
		h.logger.Error(h.logTag, "non successful response from server '%s': %v", h.address, httpResponse)
		responseMsg.SetRcode(request, dns.RcodeServerFailure)
		return responseMsg
	}

	httpDNSMessage := &httpDNSMessage{}
	bytes, err := ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		h.logger.Error(h.logTag, "failed to read response message '%s': %v", string(bytes), err)
		responseMsg.SetRcode(request, dns.RcodeServerFailure)
		return responseMsg
	}

	err = json.Unmarshal(bytes, httpDNSMessage)
	if err != nil {
		h.logger.Error(h.logTag, "failed to unmarshal response message '%s': %v", string(bytes), err)
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
