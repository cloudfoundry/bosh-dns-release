package main_test

import (
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("wait", func() {
	It("passes when the check passes", func() {
		command := exec.Command(pathToBinary, `--timeout=5s`, `--checkDomain=google.com.`)
		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session).Should(gexec.Exit(0), "running wait command failed: exit status non-zero")
	})

	It("fails when the check fails", func() {
		command := exec.Command(pathToBinary, `--timeout=5ms`, `--checkDomain=something.does-not-exist.`)
		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(session, time.Second).Should(gexec.Exit(1))
	})
})
