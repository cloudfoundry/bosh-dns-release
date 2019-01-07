package gomegadns

import (
	"fmt"

	"github.com/miekg/dns"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

func HaveFlags(flags ...string) types.GomegaMatcher {
	return &haveFlagsMatcher{
		expected: flags,
	}
}

type haveFlagsMatcher struct {
	expected []string
}

func (matcher *haveFlagsMatcher) Match(actual interface{}) (success bool, err error) {
	msg, ok := actual.(*dns.Msg)
	if !ok {
		return false, fmt.Errorf("HaveFlags matcher expects a *dns.Msg")
	}
	actualFlags := flags(msg)

	return gomega.ConsistOf(matcher.expected).Match(actualFlags)
}

func (matcher *haveFlagsMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%+v\nto have dns flags\n\t%+v. Found flags %v", actual, matcher.expected, flags(actual.(*dns.Msg)))
}

func (matcher *haveFlagsMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%+v\nnot to have dns flags\n\t%+v. Found flags %v", actual, matcher.expected, flags(actual.(*dns.Msg)))
}

var flagsNames = []string{"qr", "aa", "tc", "rd", "ra", "z", "ad", "cd"}

func flags(m *dns.Msg) []string {
	set := []string{}
	for _, f := range flagsNames {
		if fetchFlag(m, f) {
			set = append(set, f)
		}
	}
	return set
}

func fetchFlag(m *dns.Msg, flag string) bool {
	switch flag {
	case "qr":
		return m.Response
	case "aa":
		return m.Authoritative
	case "tc":
		return m.Truncated
	case "rd":
		return m.RecursionDesired
	case "ra":
		return m.RecursionAvailable
	case "z":
		return m.Zero
	case "ad":
		return m.AuthenticatedData
	case "cd":
		return m.CheckingDisabled
	default:
		panic("no DNS flag named " + flag)
	}
}
