package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
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

	pathToServer, err = gexec.Build("github.com/cloudfoundry/dns-release/src")
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})
