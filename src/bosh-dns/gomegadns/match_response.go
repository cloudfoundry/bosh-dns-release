package gomegadns

import (
	"fmt"

	"github.com/miekg/dns"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

type Response map[string]interface{}

func MatchResponse(response Response) types.GomegaMatcher {
	return &matchResponseMatcher{
		expected: response,
	}
}

type matchResponseMatcher struct {
	expected Response
}

func fetchName(m dns.RR) string {
	return m.Header().Name
}

func FetchIP(m dns.RR) string {
	if address, ok := m.(*dns.A); ok {
		return address.A.String()
	}

	return m.(*dns.PTR).String()
}

func fetchRrtype(m dns.RR) int {
	return int(m.Header().Rrtype)
}

func fetchClass(m dns.RR) int {
	return int(m.Header().Class)
}

func fetchTTL(m dns.RR) int {
	return int(m.Header().Ttl)
}

var responseFields = []string{"name", "ip", "rrtype", "class", "ttl"}

func (matcher *matchResponseMatcher) Match(actual interface{}) (success bool, err error) {
	msg, ok := actual.(dns.RR)
	if !ok {
		return false, fmt.Errorf("MatchResponse matcher expects a dns.RR")
	}
	encodedActual := map[string]interface{}{
		"name":   fetchName(msg),
		"ip":     FetchIP(msg),
		"rrtype": fetchRrtype(msg),
		"class":  fetchClass(msg),
		"ttl":    fetchTTL(msg),
	}

	matchers := []types.GomegaMatcher{}
	expected := matcher.expected
	fetch := func(k string) func(map[string]interface{}) interface{} {
		return func(e map[string]interface{}) interface{} {
			return e[k]
		}
	}

	for _, k := range responseFields {
		if v, ok := expected[k]; ok {
			matchers = append(matchers, gomega.WithTransform(fetch(k), gomega.Equal(v)))
		}
	}

	return gomega.SatisfyAll(matchers...).Match(encodedActual)
}

func (matcher *matchResponseMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%+v\nto match response\n\t%+v.", actual, matcher.expected)
}

func (matcher *matchResponseMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%+v\nnot to match response\n\t%+v.", actual, matcher.expected)
}
