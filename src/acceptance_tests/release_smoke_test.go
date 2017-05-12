package acceptance_test

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"os/exec"

	"time"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Integration", func() {
	var firstInstance instanceInfo

	BeforeEach(func() {
		firstInstance = allDeployedInstances[0]
	})

	It("returns records for bosh instances", func() {
		cmd := exec.Command("dig", strings.Split(fmt.Sprintf("-t A %s.dns.default.bosh-dns.bosh @%s", firstInstance.InstanceID, firstInstance.IP), " ")...)
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session, 10*time.Second).Should(gexec.Exit(0))
		Eventually(session.Out).Should(gbytes.Say("Got answer:"))
		Eventually(session.Out).Should(gbytes.Say("flags: qr aa rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
		Eventually(session.Out).Should(gbytes.Say(
			"%s\\.dns\\.default\\.bosh-dns\\.bosh\\.\\s+0\\s+IN\\s+A\\s+%s",
			firstInstance.InstanceID,
			firstInstance.IP))
		Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("SERVER: %s#53", firstInstance.IP)))
	})

	It("returns records for bosh instances found with query for all records", func() {
		Expect(len(allDeployedInstances)).To(BeNumerically(">", 1))

		cmd := exec.Command("dig", strings.Split(fmt.Sprintf("-t A q-YWxs.dns.default.bosh-dns.bosh @%s", firstInstance.IP), " ")...)
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session, 10*time.Second).Should(gexec.Exit(0))
		output := string(session.Out.Contents())
		Expect(output).To(ContainSubstring("Got answer:"))
		Expect(output).To(ContainSubstring("flags: qr aa rd; QUERY: 1, ANSWER: %d, AUTHORITY: 0, ADDITIONAL: 0", len(allDeployedInstances)))
		for _, info := range allDeployedInstances {
			Expect(output).To(MatchRegexp("q-YWxs\\.dns\\.default\\.bosh-dns\\.bosh\\.\\s+0\\s+IN\\s+A\\s+%s", info.IP))
		}
		Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("SERVER: %s#53", firstInstance.IP)))
	})

	It("finds and resolves aliases specified in other jobs on the same instance", func() {
		cmd := exec.Command("dig", strings.Split(fmt.Sprintf("-t A A internal.alias. @%s", firstInstance.IP), " ")...)
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session, 10*time.Second).Should(gexec.Exit(0))
		Eventually(session.Out).Should(gbytes.Say("Got answer:"))
		Eventually(session.Out).Should(gbytes.Say("flags: qr aa rd; QUERY: 1, ANSWER: %d, AUTHORITY: 0, ADDITIONAL: 0", len(allDeployedInstances)))

		Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("SERVER: %s#53", firstInstance.IP)))
	})
})
