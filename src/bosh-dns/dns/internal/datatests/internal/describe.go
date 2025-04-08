package internal

import (
	"time"

	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "bosh-dns/dns/internal/testhelpers/question_case_helpers"
)

func DescribeMatchingAnyA(serverFactory func() Server, entries ...TableEntry) bool {
	return DescribeTable(
		"matches A records",
		func(hostname string, expected ...string) {
			var r *dns.Msg
			c := &dns.Client{Net: "tcp"}

			Eventually(func() int {
				var err error

				m := &dns.Msg{}
				SetQuestion(m, nil, hostname, dns.TypeANY)
				r, _, err = c.Exchange(m, serverFactory().Bind)
				if err != nil {
					return -1
				}

				return r.Rcode
			}, 3*time.Second, time.Second).Should(Equal(dns.RcodeSuccess))

			var actual []string

			for _, answer := range r.Answer {
				actual = append(actual, answer.(*dns.A).A.String())
			}

			Expect(actual).To(ConsistOf(expected))
		},
		entries,
	)
}
