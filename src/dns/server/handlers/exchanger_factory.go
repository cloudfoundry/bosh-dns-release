package handlers

import (
	"time"

	"github.com/miekg/dns"
)

type ExchangerFactory func(string) Exchanger

func NewExchangerFactory(timeout time.Duration) ExchangerFactory {
	return func(net string) Exchanger {
		return &dns.Client{Net: net, Timeout: timeout}
	}
}
