package dnsresolver_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestDnsresolver(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "dns/server/records/dnsresolver")
}
