package main_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

func TestDebugCli(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "cli")
}

var (
	pathToCli string
)

var _ = SynchronizedBeforeSuite(func() []byte {
	path, err := gexec.Build("debug/cli")
	Expect(err).NotTo(HaveOccurred())
	SetDefaultEventuallyTimeout(2 * time.Second)
	return []byte(path)
}, func(data []byte) {
	pathToCli = string(data)
})

var _ = SynchronizedAfterSuite(func() {
}, func() {
	gexec.CleanupBuildArtifacts()
})
