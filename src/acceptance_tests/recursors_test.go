package acceptance_test

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	"strings"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"runtime"
)

var _ = Describe("recursor", func() {
	var (
		recursorSession *gexec.Session
		firstInstance   instanceInfo
		err             error
	)

	Context("when the recursors must be read from the system resolver list", func() {
		BeforeEach(func() {
			cmd := exec.Command(pathToTestRecursorServer, "53")
			recursorSession, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			ensureRecursorIsDefinedByBoshAgent()
			firstInstance = allDeployedInstances[0]
		})

		AfterEach(func() {
			recursorSession.Kill()
		})

		It("fowards queries to the configured recursors on port 53", func() {
			if runtime.GOOS == "windows" {
				Skip("Windows agent does not properly configure DNS nameservers from cloud config")
			}

			cmd := exec.Command("dig", strings.Split(fmt.Sprintf("-t A example.com @%s", firstInstance.IP), " ")...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session, 10*time.Second).Should(gexec.Exit(0))
			output := string(session.Out.Contents())
			Expect(output).To(ContainSubstring("flags: qr aa rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
			Expect(output).To(MatchRegexp("example.com.\\s+0\\s+IN\\s+A\\s+10\\.10\\.10\\.10"))
			Expect(output).To(ContainSubstring(fmt.Sprintf("SERVER: %s#53", firstInstance.IP)))
		})
	})

	Context("when the recursors are configured explicitly on the DNS server", func() {
		BeforeEach(func() {
			cmd := exec.Command(pathToTestRecursorServer, "9955")
			recursorSession, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			ensureRecursorIsDefinedByDnsRelease()
			firstInstance = allDeployedInstances[0]
		})

		AfterEach(func() {
			recursorSession.Kill()
		})

		It("returns success when receiving a truncated responses from a recursor", func() {
			By("ensuring the test recursor is returning truncated messages", func() {
				cmd := exec.Command("dig", strings.Split("+ignore +notcp -p 9955 -t A truncated-recursor.com. @127.0.0.1", " ")...)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session, 10*time.Second).Should(gexec.Exit(0))
				output := string(session.Out.Contents())
				Expect(output).To(ContainSubstring(";; flags: qr aa tc rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
			})

			By("ensuring the dns release returns a successful truncated recursed answer", func() {
				cmd := exec.Command("dig", strings.Split(fmt.Sprintf("+ignore +notcp -t A truncated-recursor.com. @%s", firstInstance.IP), " ")...)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session, 10*time.Second).Should(gexec.Exit(0))
				output := string(session.Out.Contents())
				Expect(output).To(ContainSubstring(";; flags: qr aa tc rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
			})
		})

		It("timeouts when recursor takes longer than configured recursor_timeout", func() {
			By("ensuring the test recursor is working", func() {
				cmd := exec.Command("dig", strings.Split("+ignore +notcp -p 9955 -t A slow-recursor.com. @127.0.0.1", " ")...)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session, 10*time.Second).Should(gexec.Exit(0))
				output := string(session.Out.Contents())
				Expect(output).To(ContainSubstring(";; flags: qr aa rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
			})

			By("ensuring the dns release returns a error due to recursor timing out", func() {
				cmd := exec.Command("dig", strings.Split(fmt.Sprintf("+ignore +notcp -t A slow-recursor.com. @%s", firstInstance.IP), " ")...)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session, 10*time.Second).Should(gexec.Exit(0))
				output := string(session.Out.Contents())
				Expect(output).To(ContainSubstring("status: SERVFAIL"))
			})
		})

		It("forwards large UDP EDNS messages", func() {
			By("ensuring the test recursor is returning messages", func() {
				cmd := exec.Command("dig", strings.Split("+ignore +notcp +bufsize=65535 -p 9955 udp-9k-message.com. @127.0.0.1", " ")...)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session, 10*time.Second).Should(gexec.Exit(0))
				output := string(session.Out.Contents())
				Expect(output).To(ContainSubstring(";; flags: qr aa rd; QUERY: 1, ANSWER: 270, AUTHORITY: 0, ADDITIONAL: 0"))
				Expect(output).To(ContainSubstring("MSG SIZE  rcvd: 9156"))
			})

			By("ensuring the dns release returns a successful trucated recursed answer", func() {
				cmd := exec.Command("dig", strings.Split(fmt.Sprintf("+ignore +notcp +bufsize=65535 udp-9k-message.com. @%s", firstInstance.IP), " ")...)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session, 10*time.Second).Should(gexec.Exit(0))
				output := string(session.Out.Contents())
				Expect(output).To(ContainSubstring(";; flags: qr aa rd; QUERY: 1, ANSWER: 270, AUTHORITY: 0, ADDITIONAL: 0"))
				Expect(output).To(ContainSubstring("MSG SIZE  rcvd: 9156"))
			})
		})

		It("compresses message responses that are larger than requested UDPSize", func() {
			By("ensuring the test recursor is returning messages", func() {
				cmd := exec.Command("dig", strings.Split("+ignore +notcp +bufsize=16384 -p 9955 compressed-ip-truncated-recursor-large.com. @127.0.0.1", " ")...)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session, 10*time.Second).Should(gexec.Exit(0))
				output := string(session.Out.Contents())
				Expect(output).To(ContainSubstring(";; flags: qr aa rd; QUERY: 1, ANSWER: 512, AUTHORITY: 0, ADDITIONAL: 0"))
				Expect(output).To(ContainSubstring("MSG SIZE  rcvd: 7224"))
			})

			By("ensuring the dns release returns a successful compressed recursed answer", func() {
				cmd := exec.Command("dig", strings.Split(fmt.Sprintf("+ignore +notcp +bufsize=16384 compressed-ip-truncated-recursor-large.com. @%s", firstInstance.IP), " ")...)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session, 10*time.Second).Should(gexec.Exit(0))
				output := string(session.Out.Contents())
				Expect(output).To(ContainSubstring(";; flags: qr aa rd; QUERY: 1, ANSWER: 512, AUTHORITY: 0, ADDITIONAL: 0"))
				Expect(output).To(ContainSubstring("MSG SIZE  rcvd: 7224"))
			})
		})

		It("forwards large dns answers even if udp response size is larger than 512", func() { // this test drove out the UDPSize on the client produced in the ClientExchangeFactory
			By("ensuring the test recursor is returning messages", func() {
				cmd := exec.Command("dig", strings.Split("+ignore +notcp -p 9955 ip-truncated-recursor-large.com. @127.0.0.1", " ")...)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session, 10*time.Second).Should(gexec.Exit(0))
				output := string(session.Out.Contents())
				Expect(output).To(ContainSubstring(";; flags: qr aa tc rd; QUERY: 1, ANSWER: 20, AUTHORITY: 0, ADDITIONAL: 0"))
				Expect(output).To(ContainSubstring("MSG SIZE  rcvd: 989"))
			})

			By("ensuring the dns release returns a successful trucated recursed answer", func() {
				cmd := exec.Command("dig", strings.Split(fmt.Sprintf("+ignore +notcp ip-truncated-recursor-large.com. @%s", firstInstance.IP), " ")...)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session, 10*time.Second).Should(gexec.Exit(0))
				output := string(session.Out.Contents())
				Expect(output).To(ContainSubstring(";; flags: qr aa tc rd; QUERY: 1, ANSWER: 20, AUTHORITY: 0, ADDITIONAL: 0"))
				Expect(output).To(ContainSubstring("MSG SIZE  rcvd: 989"))
			})
		})

		It("does not bother to compress messages that are smaller than 512", func() {
			By("ensuring the test recursor is returning messages", func() {
				cmd := exec.Command("dig", strings.Split("+ignore +bufsize=1 +notcp -p 9955 recursor-small.com. @127.0.0.1", " ")...)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session, 10*time.Second).Should(gexec.Exit(0))
				output := string(session.Out.Contents())
				Expect(output).To(ContainSubstring(";; flags: qr aa rd; QUERY: 1, ANSWER: 2, AUTHORITY: 0, ADDITIONAL: 0"))
				Expect(output).To(ContainSubstring("MSG SIZE  rcvd: 104"))
			})

			By("ensuring the dns release returns a successful trucated recursed answer", func() {
				cmd := exec.Command("dig", strings.Split(fmt.Sprintf("+ignore +notcp recursor-small.com. @%s", firstInstance.IP), " ")...)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session, 10*time.Second).Should(gexec.Exit(0))
				output := string(session.Out.Contents())
				Expect(output).To(ContainSubstring(";; flags: qr aa rd; QUERY: 1, ANSWER: 2, AUTHORITY: 0, ADDITIONAL: 0"))
				Expect(output).To(ContainSubstring("MSG SIZE  rcvd: 104"))
			})
		})

		It("fowards queries to the configured recursors", func() {
			cmd := exec.Command("dig", strings.Split(fmt.Sprintf("-t A example.com @%s", firstInstance.IP), " ")...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session, 10*time.Second).Should(gexec.Exit(0))
			output := string(session.Out.Contents())
			Expect(output).To(ContainSubstring("flags: qr aa rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
			Expect(output).To(MatchRegexp("example.com.\\s+0\\s+IN\\s+A\\s+10\\.10\\.10\\.10"))
			Expect(output).To(ContainSubstring(fmt.Sprintf("SERVER: %s#53", firstInstance.IP)))
		})
	})
})

func ensureRecursorIsDefinedByBoshAgent() {
	cmdRunner = system.NewExecCmdRunner(boshlog.NewLogger(boshlog.LevelDebug))

	manifestPath, err := filepath.Abs(fmt.Sprintf("../test_yml_assets/%s.yml", manifestName))
	Expect(err).ToNot(HaveOccurred())
	disableOverridePath, err := filepath.Abs(fmt.Sprintf("../test_yml_assets/%s.yml", noRecursorsOpsFile))
	Expect(err).ToNot(HaveOccurred())
	aliasProvidingPath, err := filepath.Abs("dns-acceptance-release")
	Expect(err).ToNot(HaveOccurred())

	updateCloudConfigWithOurLocalRecursor()

	stdOut, stdErr, exitStatus, err := cmdRunner.RunCommand(boshBinaryPath,
		"-n", "-d", boshDeployment, "deploy",
		"-v", fmt.Sprintf("name=%s", boshDeployment),
		"-v", fmt.Sprintf("acceptance_release_path=%s", aliasProvidingPath),
		"-o", disableOverridePath,
		manifestPath,
	)
	Expect(err).ToNot(HaveOccurred())
	Expect(exitStatus).To(Equal(0), fmt.Sprintf("stdOut: %s \n stdErr: %s", stdOut, stdErr))
	allDeployedInstances = getInstanceInfos(boshBinaryPath)
}

func ensureRecursorIsDefinedByDnsRelease() {
	cmdRunner = system.NewExecCmdRunner(boshlog.NewLogger(boshlog.LevelDebug))

	manifestPath, err := filepath.Abs(fmt.Sprintf("../test_yml_assets/%s.yml", manifestName))
	Expect(err).ToNot(HaveOccurred())
	aliasProvidingPath, err := filepath.Abs("dns-acceptance-release")
	Expect(err).ToNot(HaveOccurred())

	updateCloudConfigWithDefaultCloudConfig()

	stdOut, stdErr, exitStatus, err := cmdRunner.RunCommand(boshBinaryPath,
		"-n", "-d", boshDeployment, "deploy",
		"-v", fmt.Sprintf("name=%s", boshDeployment),
		"-v", fmt.Sprintf("acceptance_release_path=%s", aliasProvidingPath),
		manifestPath,
	)
	Expect(err).ToNot(HaveOccurred())
	Expect(exitStatus).To(Equal(0), fmt.Sprintf("stdOut: %s \n stdErr: %s", stdOut, stdErr))
	allDeployedInstances = getInstanceInfos(boshBinaryPath)
}

func updateCloudConfigWithOurLocalRecursor() {
	removeRecursorAddressesOpsFile, err := filepath.Abs(fmt.Sprintf("../test_yml_assets/%s.yml", setupLocalRecursorOpsFile))
	Expect(err).ToNot(HaveOccurred())
	stdOut, stdErr, exitStatus, err := cmdRunner.RunCommand(boshBinaryPath, "-n", "update-cloud-config", "-o", removeRecursorAddressesOpsFile, "-v", "network=director_network", cloudConfigTempFileName)
	Expect(err).ToNot(HaveOccurred())
	Expect(exitStatus).To(Equal(0), fmt.Sprintf("stdOut: %s \n stdErr: %s", stdOut, stdErr))
}

func updateCloudConfigWithDefaultCloudConfig() {
	stdOut, stdErr, exitStatus, err := cmdRunner.RunCommand(boshBinaryPath, "-n", "update-cloud-config", "-v", "network=director_network", cloudConfigTempFileName)
	Expect(err).ToNot(HaveOccurred())
	Expect(exitStatus).To(Equal(0), fmt.Sprintf("stdOut: %s \n stdErr: %s", stdOut, stdErr))
}
