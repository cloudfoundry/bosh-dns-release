package handlers

import (
	"errors"
	"github.com/cloudfoundry/dns-release/src/dns/server/aliases"
	"github.com/miekg/dns"
)

type AliasResolvingHandler struct {
	child  dns.Handler
	config aliases.Config
}

func NewAliasResolvingHandler(child dns.Handler, config aliases.Config) (AliasResolvingHandler, error) {
	if !config.IsReduced() {
		return AliasResolvingHandler{}, errors.New("must configure with non-recursing alias config")
	}

	return AliasResolvingHandler{
		child:  child,
		config: config,
	}, nil
}

func (h AliasResolvingHandler) ServeDNS(resp dns.ResponseWriter, msg *dns.Msg) {

	resolvedQuestions := []dns.Question{}

	for _, question := range msg.Question {
		for _, resolution := range h.config.Resolutions(question.Name) {
			resolvedQuestions = append(resolvedQuestions, dns.Question{
				Name:   resolution,
				Qtype:  question.Qtype,
				Qclass: question.Qclass,
			})
		}
	}

	resolvedMsg := dns.Msg{
		MsgHdr:   msg.MsgHdr,
		Compress: msg.Compress,
		Question: resolvedQuestions,
	}

	h.child.ServeDNS(resp, &resolvedMsg)
}
