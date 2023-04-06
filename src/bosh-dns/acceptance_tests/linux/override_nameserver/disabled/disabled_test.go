//go:build linux || darwin
// +build linux darwin

package override_nameserver

import (
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gexec"
)

var _ = Describe("dns job: override_nameserver", func() {
	Describe("disabled", func() {
		It("does not resolve the bosh-dns upcheck", func() {
			for i := 0; i < 5; i++ {
				cmd := exec.Command(boshBinaryPath, []string{"ssh", "-d", boshDeployment, "bosh-dns/0", "-c", "dig +time=3 -t A upcheck.bosh-dns."}...)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session, 20*time.Second).Should(gexec.Exit())
				output := string(session.Out.Contents())
				Expect(output).To(ContainSubstring("status: NXDOMAIN"))
				time.Sleep(400 * time.Millisecond)
			}
		})
	})
})
