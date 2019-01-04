package acceptance

import (
	"bosh-dns/acceptance_tests/helpers"
	"fmt"
	"os/exec"

	"strings"

	"regexp"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

const testRecursorAddress = "@127.0.0.1"

var _ = Describe("recursor", func() {
	var (
		recursorSession *gexec.Session
		firstInstance   helpers.InstanceInfo
	)

	Context("when the recursors must be read from the system resolver list", func() {
		BeforeEach(func() {
			var err error
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
			cmd := exec.Command("dig",
				"-t", "A",
				"example.com", fmt.Sprintf("@%s", firstInstance.IP),
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			<-session.Exited
			Expect(session.ExitCode()).To(BeZero())

			output := string(session.Out.Contents())
			Expect(output).To(ContainSubstring("flags: qr aa rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
			Expect(output).To(MatchRegexp("example.com.\\s+5\\s+IN\\s+A\\s+10\\.10\\.10\\.10"))
			Expect(output).To(ContainSubstring(fmt.Sprintf("SERVER: %s#53", firstInstance.IP)))
		})
	})

	Context("when the recursors are configured explicitly on the DNS server", func() {
		BeforeEach(func() {
			var err error
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
				cmd := exec.Command("dig",
					"+ignore", "+notcp", "-p", "9955", "-t", "A",
					"truncated-recursor.com.", testRecursorAddress,
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-session.Exited
				Expect(session.ExitCode()).To(BeZero())

				output := string(session.Out.Contents())
				Expect(output).To(ContainSubstring(";; flags: qr aa tc rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
			})

			By("ensuring the dns release returns a successful truncated recursed answer", func() {
				cmd := exec.Command("dig",
					"+ignore", "+notcp", "-t", "A",
					"truncated-recursor.com.", fmt.Sprintf("@%s", firstInstance.IP),
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-session.Exited
				Expect(session.ExitCode()).To(BeZero())

				output := string(session.Out.Contents())
				Expect(output).To(ContainSubstring(";; flags: qr aa tc rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
			})
		})

		It("timeouts when recursor takes longer than configured recursor_timeout", func() {
			By("ensuring the test recursor is working", func() {
				cmd := exec.Command("dig",
					"+ignore", "+notcp", "-p", "9955",
					"-t", "A", "slow-recursor.com.", testRecursorAddress,
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-session.Exited
				Expect(session.ExitCode()).To(BeZero())

				output := string(session.Out.Contents())
				Expect(output).To(ContainSubstring(";; flags: qr aa rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
			})

			By("ensuring the dns release returns a error due to recursor timing out", func() {
				cmd := exec.Command("dig",
					"+ignore", "+notcp", "-t", "A", "slow-recursor.com.",
					fmt.Sprintf("@%s", firstInstance.IP),
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-session.Exited
				Expect(session.ExitCode()).To(BeZero())

				output := string(session.Out.Contents())
				Expect(output).To(ContainSubstring("status: SERVFAIL"))
			})
		})

		It("forwards large UDP EDNS messages", func() {
			By("ensuring the test recursor is returning messages", func() {
				cmd := exec.Command("dig",
					"+ignore", "+notcp", "+bufsize=65535", "-p", "9955",
					"udp-9k-message.com.", testRecursorAddress,
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-session.Exited
				Expect(session.ExitCode()).To(BeZero())

				output := string(session.Out.Contents())
				Expect(output).To(ContainSubstring(";; flags: qr aa rd ra; QUERY: 1, ANSWER: 270, AUTHORITY: 0, ADDITIONAL: 0"))
				Expect(output).To(ContainSubstring("MSG SIZE  rcvd: 9156"))
			})

			By("ensuring the dns release returns a successful trucated recursed answer", func() {
				cmd := exec.Command("dig",
					"+ignore", "+notcp", "+bufsize=65535",
					"udp-9k-message.com.", fmt.Sprintf("@%s", firstInstance.IP),
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-session.Exited
				Expect(session.ExitCode()).To(BeZero())

				output := string(session.Out.Contents())
				Expect(output).To(ContainSubstring(";; flags: qr aa rd ra; QUERY: 1, ANSWER: 270, AUTHORITY: 0, ADDITIONAL: 0"))
				Expect(output).To(ContainSubstring("MSG SIZE  rcvd: 9156"))
			})
		})

		It("compresses message responses that are larger than requested UDPSize", func() {
			By("ensuring the test recursor is returning messages", func() {
				cmd := exec.Command("dig",
					"+ignore", "+notcp", "+bufsize=16384", "-p", "9955",
					"compressed-ip-truncated-recursor-large.com.", testRecursorAddress,
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-session.Exited
				Expect(session.ExitCode()).To(BeZero())

				output := string(session.Out.Contents())
				Expect(output).To(ContainSubstring(";; flags: qr aa rd ra; QUERY: 1, ANSWER: 512, AUTHORITY: 0, ADDITIONAL: 0"))
				Expect(output).To(ContainSubstring("MSG SIZE  rcvd: 7224"))
			})

			By("ensuring the dns release returns a successful compressed recursed answer", func() {
				cmd := exec.Command("dig",
					"+ignore", "+notcp", "+bufsize=16384",
					"compressed-ip-truncated-recursor-large.com.", fmt.Sprintf("@%s", firstInstance.IP),
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-session.Exited
				Expect(session.ExitCode()).To(BeZero())

				output := string(session.Out.Contents())
				Expect(output).To(ContainSubstring(";; flags: qr aa rd ra; QUERY: 1, ANSWER: 512, AUTHORITY: 0, ADDITIONAL: 0"))
				Expect(output).To(ContainSubstring("MSG SIZE  rcvd: 7224"))
			})
		})

		It("forwards large dns answers even if udp response size is larger than 512", func() {
			By("ensuring the test recursor is returning messages", func() {
				cmd := exec.Command("dig",
					"+ignore", "+notcp", "-p", "9955",
					"ip-truncated-recursor-large.com.", testRecursorAddress,
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-session.Exited
				Expect(session.ExitCode()).To(BeZero())

				output := string(session.Out.Contents())
				Expect(output).To(ContainSubstring(";; flags: qr aa tc rd ra; QUERY: 1, ANSWER: 20, AUTHORITY: 0, ADDITIONAL: 0"))
				Expect(output).To(ContainSubstring("MSG SIZE  rcvd: 989"))
			})

			By("ensuring the dns release returns a successful trucated recursed answer", func() {
				cmd := exec.Command("dig",
					"+ignore", "+notcp",
					"ip-truncated-recursor-large.com.", fmt.Sprintf("@%s", firstInstance.IP),
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-session.Exited
				Expect(session.ExitCode()).To(BeZero())

				output := string(session.Out.Contents())
				Expect(output).To(ContainSubstring(";; flags: qr aa tc rd ra; QUERY: 1, ANSWER: 20, AUTHORITY: 0, ADDITIONAL: 0"))
				Expect(output).To(ContainSubstring("MSG SIZE  rcvd: 989"))
			})
		})

		It("does not bother to compress messages that are smaller than 512", func() {
			By("ensuring the test recursor is returning messages", func() {
				cmd := exec.Command("dig",
					"+ignore", "+bufsize=1", "+notcp", "-p", "9955",
					"recursor-small.com.", testRecursorAddress,
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-session.Exited
				Expect(session.ExitCode()).To(BeZero())

				output := string(session.Out.Contents())
				Expect(output).To(ContainSubstring(";; flags: qr aa rd ra; QUERY: 1, ANSWER: 2, AUTHORITY: 0, ADDITIONAL: 0"))
				Expect(output).To(ContainSubstring("MSG SIZE  rcvd: 104"))
			})

			By("ensuring the dns release returns a successful trucated recursed answer", func() {
				cmd := exec.Command("dig",
					"+ignore", "+notcp",
					"recursor-small.com.", fmt.Sprintf("@%s", firstInstance.IP),
				)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-session.Exited
				Expect(session.ExitCode()).To(BeZero())

				output := string(session.Out.Contents())
				Expect(output).To(ContainSubstring(";; flags: qr aa rd ra; QUERY: 1, ANSWER: 2, AUTHORITY: 0, ADDITIONAL: 0"))
				Expect(output).To(ContainSubstring("MSG SIZE  rcvd: 104"))
			})
		})

		It("fowards queries to the configured recursors", func() {
			cmd := exec.Command("dig", strings.Split(fmt.Sprintf("-t A example.com @%s", firstInstance.IP), " ")...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			<-session.Exited
			Expect(session.ExitCode()).To(BeZero())

			output := string(session.Out.Contents())
			Expect(output).To(ContainSubstring("flags: qr aa rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
			Expect(output).To(MatchRegexp("example.com.\\s+5\\s+IN\\s+A\\s+10\\.10\\.10\\.10"))
			Expect(output).To(ContainSubstring(fmt.Sprintf("SERVER: %s#53", firstInstance.IP)))
		})

		It("fowards ipv4 ARPA queries to the configured recursors", func() {
			cmd := exec.Command("dig", strings.Split(fmt.Sprintf("-x 8.8.4.4 @%s", firstInstance.IP), " ")...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			<-session.Exited
			Expect(session.ExitCode()).To(BeZero())

			output := string(session.Out.Contents())
			Expect(output).To(ContainSubstring("flags: qr aa rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
			Expect(output).To(MatchRegexp(`4\.4\.8\.8\.in-addr\.arpa\.\s+\d+\s+IN\s+PTR\s+google-public-dns-b\.google\.com\.`))
			Expect(output).To(ContainSubstring(fmt.Sprintf("SERVER: %s#53", firstInstance.IP)))
		})

		It("fowards ipv6 ARPA queries to the configured recursors", func() {
			cmd := exec.Command("dig", strings.Split(fmt.Sprintf("-x 2001:4860:4860::8888 @%s", firstInstance.IP), " ")...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			<-session.Exited
			Expect(session.ExitCode()).To(BeZero())

			output := string(session.Out.Contents())
			Expect(output).To(ContainSubstring("flags: qr aa rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
			Expect(output).To(MatchRegexp(`8\.8\.8\.8\.0\.0\.0\.0\.0\.0\.0\.0\.0\.0\.0\.0\.0\.0\.0\.0\.0\.6\.8\.4\.0\.6\.8\.4\.1\.0\.0\.2\.ip6\.arpa\.\s+\d+\s+IN\s+PTR\s+google-public-dns-a\.google\.com\.`))
			Expect(output).To(ContainSubstring(fmt.Sprintf("SERVER: %s#53", firstInstance.IP)))
		})
	})

	Context("when using cache", func() {
		BeforeEach(func() {
			var err error
			cmd := exec.Command(pathToTestRecursorServer, "9955")
			recursorSession, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			ensureRecursorIsDefinedByDnsRelease()
			firstInstance = allDeployedInstances[0]
		})

		AfterEach(func() {
			recursorSession.Kill()
		})

		It("caches dns recursed dns entries for the duration of the TTL", func() {
			cmd := exec.Command("dig",
				"+notcp", "always-different-with-timeout-example.com.",
				fmt.Sprintf("@%s", firstInstance.IP),
			)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			<-session.Exited
			Expect(session.ExitCode()).To(BeZero())

			output := string(session.Out.Contents())
			re := regexp.MustCompile("\\s+(\\d+)\\s+IN\\s+A\\s+127\\.0\\.0\\.(\\d+)")
			matches := re.FindStringSubmatch(output)
			Expect(matches[1]).To(Equal("5"))
			Expect(matches[2]).To(Equal("1"))

			Consistently(func() string {
				cmd = exec.Command("dig",
					"+notcp", "always-different-with-timeout-example.com.",
					fmt.Sprintf("@%s", firstInstance.IP),
				)
				session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-session.Exited
				Expect(session.ExitCode()).To(BeZero())

				output = string(session.Out.Contents())

				matches = re.FindStringSubmatch(output)
				return matches[2]
			}, "4s", "1s").Should(Equal("1"))

			Eventually(func() string {
				cmd = exec.Command("dig",
					"+notcp", "always-different-with-timeout-example.com.",
					fmt.Sprintf("@%s", firstInstance.IP),
				)
				session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-session.Exited
				Expect(session.ExitCode()).To(BeZero())

				output = string(session.Out.Contents())
				matches = re.FindStringSubmatch(output)
				matches = re.FindStringSubmatch(output)
				return matches[2]
			}, "4s", "1s").Should(Equal("2"))
		})
	})
})
