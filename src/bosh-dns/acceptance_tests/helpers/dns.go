package helpers

import (
	"github.com/miekg/dns"
	. "github.com/onsi/gomega"
)

func Dig(domain, server string) *dns.Msg {
	c := &dns.Client{}
	m := &dns.Msg{}
	m.SetQuestion(domain, dns.TypeA)
	r, _, err := c.Exchange(m, server+":53")
	Expect(err).NotTo(HaveOccurred())
	Expect(r.Rcode).To(Equal(dns.RcodeSuccess))
	return r
}
