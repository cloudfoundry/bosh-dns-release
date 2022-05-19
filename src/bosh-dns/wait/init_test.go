package main_test

import (
	"testing"
	"time"

	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestWait(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "wait")
}

var (
	pathToBinary string
)

var _ = SynchronizedBeforeSuite(func() []byte {
	waitPath, err := gexec.Build("bosh-dns/wait")
	Expect(err).NotTo(HaveOccurred())
	SetDefaultEventuallyTimeout(2 * time.Second)
	SetDefaultEventuallyPollingInterval(500 * time.Millisecond)

	return []byte(waitPath)
}, func(data []byte) {
	pathToBinary = string(data)
})

var _ = SynchronizedAfterSuite(func() {
}, func() {
	gexec.CleanupBuildArtifacts()
})
