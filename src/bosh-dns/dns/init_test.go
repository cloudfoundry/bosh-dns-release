package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
	"time"
)

func TestDNS(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "dns")
}

var (
	pathToServer string
)

var _ = BeforeSuite(func() {
	var err error

	pathToServer, err = gexec.Build("bosh-dns/dns")
	Expect(err).NotTo(HaveOccurred())
	SetDefaultEventuallyTimeout(2 * time.Second)
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})
