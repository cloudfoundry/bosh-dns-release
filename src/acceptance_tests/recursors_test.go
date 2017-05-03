package acceptance_test

import (
	"fmt"
	"os/exec"
	"time"

	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("recursor", func() {
	var (
		session       *gexec.Session
		firstInstance instanceInfo
	)

	BeforeEach(func() {
		var err error
		cmd := exec.Command(pathToTestRecursorServer)
		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		firstInstance = allDeployedInstances[0]
	})

	AfterEach(func() {
		session.Kill()
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

		By("ensuring the dns release returns a successful trucated recursed answer", func() {
			cmd := exec.Command("dig", strings.Split(fmt.Sprintf("+ignore +notcp -t A truncated-recursor.com. @%s", firstInstance.IP), " ")...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session, 10*time.Second).Should(gexec.Exit(0))
			output := string(session.Out.Contents())
			Expect(output).To(ContainSubstring(";; flags: qr aa tc rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
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

	It("it compresses message responses that are larger than requested UDPSize", func() {
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
