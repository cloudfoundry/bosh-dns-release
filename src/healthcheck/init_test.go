package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
	"time"
)

func TestHealthCheck(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "healthcheck")
}

var (
	pathToServer string
)

var _ = BeforeSuite(func() {
	var err error

	pathToServer, err = gexec.Build("github.com/cloudfoundry/dns-release/src/healthcheck")
	Expect(err).NotTo(HaveOccurred())
	SetDefaultEventuallyTimeout(2 * time.Second)
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})
