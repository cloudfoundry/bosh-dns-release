package handlers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"

	"github.com/cloudfoundry/bosh-utils/httpclient"
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/miekg/dns"
)

type HTTPJSONHandler struct {
	address string
	client  *httpclient.HTTPClient
	logger  logger.Logger
	logTag  string
}

type ipsResponse struct {
	IPs []string `json:ips`
}

func NewHTTPJSONHandler(address string, logger logger.Logger) HTTPJSONHandler {
	return HTTPJSONHandler{
		address: address,
		client: httpclient.NewHTTPClient(
			httpclient.DefaultClient,
			logger,
		),
		logger: logger,
		logTag: "HTTPJSONHandler",
	}
}

func (h HTTPJSONHandler) ServeDNS(responseWriter dns.ResponseWriter, request *dns.Msg) {
	responseMsg := h.buildResponse(request)
	if err := responseWriter.WriteMsg(responseMsg); err != nil {
		h.logger.Error(h.logTag, err.Error())
	}
}

func (h HTTPJSONHandler) buildResponse(request *dns.Msg) *dns.Msg {
	responseMsg := new(dns.Msg)
	responseMsg.Authoritative = true
	responseMsg.RecursionAvailable = false
	responseMsg.SetReply(request)

	if len(request.Question) == 0 {
		return responseMsg
	}

	url := fmt.Sprintf("%s/ips/%s", h.address, request.Question[0].Name)
	httpResponse, err := h.client.Get(url)
	if err != nil {
		h.logger.Error(h.logTag, "Error connecting to '%s': %v", h.address, err)
		responseMsg.SetRcode(request, dns.RcodeServerFailure)
		return responseMsg
	}

	if httpResponse.StatusCode != 200 {
		h.logger.Error(h.logTag, "Non successful response from server '%s': %v", h.address, httpResponse)
		responseMsg.SetRcode(request, dns.RcodeServerFailure)
		return responseMsg
	}

	ipsResponsePayload := &ipsResponse{}
	bytes, err := ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		h.logger.Error(h.logTag, "failed to read response message '%s': %v", string(bytes), err)
		responseMsg.SetRcode(request, dns.RcodeServerFailure)
		return responseMsg
	}

	err = json.Unmarshal(bytes, ipsResponsePayload)
	if err != nil {
		h.logger.Error(h.logTag, "failed to unmarshal response message '%s': %v", string(bytes), err)
		responseMsg.SetRcode(request, dns.RcodeServerFailure)
		return responseMsg
	}

	for _, ip := range ipsResponsePayload.IPs {
		responseMsg.Answer = append(responseMsg.Answer, &dns.A{
			Hdr: dns.RR_Header{
				Name:   request.Question[0].Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    0,
			},
			A: net.ParseIP(ip),
		})
	}

	responseMsg.SetRcode(request, dns.RcodeSuccess)
	return responseMsg
}
