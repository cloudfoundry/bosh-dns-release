// +build linux darwin

package linux_test

import (
	"errors"
	"os/exec"
	"regexp"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Alias address binding", func() {
	It("should start a dns server on port 53", func() {
		cmd := exec.Command(boshBinaryPath, []string{"ssh", firstInstanceSlug, "-c", "sudo lsof -n -i :53"}...)
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session, 10*time.Second).Should(gexec.Exit(0))
		output := string(session.Out.Contents())
		Expect(output).To(MatchRegexp("dns.*TCP .*:domain"))
		Expect(output).To(MatchRegexp("dns.*UDP .*:domain"))
	})

	It("should respond to tcp dns queries", func() {
		cmd := exec.Command(boshBinaryPath, []string{"ssh", firstInstanceSlug, "-c", "dig +tcp healthcheck.bosh-dns. @169.254.0.2"}...)
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session, 10*time.Second).Should(gexec.Exit(0))
		Eventually(session.Out).Should(gbytes.Say("Got answer:"))
		Eventually(session.Out).Should(gbytes.Say("flags: qr aa rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
		Eventually(session.Out).Should(gbytes.Say("healthcheck\\.bosh-dns\\.\\s+0\\s+IN\\s+A\\s+127\\.0\\.0\\.1"))
		Eventually(session.Out).Should(gbytes.Say("SERVER: 169.254.0.2#53"))
	})

	It("should respond to udp dns queries", func() {
		cmd := exec.Command(boshBinaryPath, []string{"ssh", firstInstanceSlug, "-c", "dig +notcp healthcheck.bosh-dns. @169.254.0.2"}...)
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session, 10*time.Second).Should(gexec.Exit(0))
		Eventually(session.Out).Should(gbytes.Say("Got answer:"))
		Eventually(session.Out).Should(gbytes.Say("flags: qr aa rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
		Eventually(session.Out).Should(gbytes.Say(";healthcheck\\.bosh-dns\\.\\s+IN\\s+A"))
		Eventually(session.Out).Should(gbytes.Say("SERVER: 169.254.0.2#53"))
	})

	Context("when the healtcheck becomes unreachable", func() {
		AfterEach(func() {
			cmd := exec.Command(boshBinaryPath, []string{"ssh", firstInstanceSlug, "-c", "sudo /sbin/iptables -D INPUT -p udp -j DROP"}...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session, 10*time.Second).Should(gexec.Exit())
		})

		It("should kill itself if its healthcheck becomes unreachable", func() {
			serverPidRegex := regexp.MustCompile(`dns\S*\s+(\d+).*TCP .*:domain`)

			getServerPid := func() (int, error) {
				lsofCmd := exec.Command(boshBinaryPath, []string{"ssh", firstInstanceSlug, "-c", "sudo lsof -n -i :53"}...)
				session, err := gexec.Start(lsofCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session, 10*time.Second).Should(gexec.Exit())

				out := string(session.Out.Contents())
				match := serverPidRegex.FindStringSubmatch(out)

				if len(match) < 2 {
					return -1, errors.New("no matches found")
				}

				return strconv.Atoi(match[1])
			}
			originalServerPid, err := getServerPid()
			Expect(err).NotTo(HaveOccurred())

			cmd := exec.Command(boshBinaryPath, []string{"ssh", firstInstanceSlug, "-c", "sudo /sbin/iptables -A INPUT -p udp -j DROP"}...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session, 10*time.Second).Should(gexec.Exit(0))

			time.Sleep(time.Second * 40)

			_, err = getServerPid()
			Expect(err).To(HaveOccurred())

			cmd = exec.Command(boshBinaryPath, []string{"ssh", firstInstanceSlug, "-c", "sudo /sbin/iptables -D INPUT -p udp -j DROP"}...)
			session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session, 10*time.Second).Should(gexec.Exit(0))

			time.Sleep(time.Second * 9)

			pid, err := getServerPid()
			Expect(err).NotTo(HaveOccurred())
			Expect(pid).NotTo(Equal(originalServerPid))
		})
	})

	Context("as the system-configured nameserver", func() {
		It("resolves the bosh-dns healthcheck", func() {
			cmd := exec.Command(boshBinaryPath, []string{"ssh", "dns/0", "-c", "dig -t A healthcheck.bosh-dns."}...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session, 10*time.Second).Should(gexec.Exit(0))
			output := string(session.Out.Contents())
			Expect(output).To(ContainSubstring(";; flags: qr aa rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
		})

		Context("external processes changing /etc/resolv.conf", func() {
			BeforeEach(func() {
				backup := exec.Command(boshBinaryPath, []string{"ssh", "dns/0", "-c", "sudo cp `readlink /etc/resolv.conf` /tmp/resolv.conf.backup"}...)

				session, err := gexec.Start(backup, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session, 10*time.Second).Should(gexec.Exit(0))
			})

			AfterEach(func() {
				restore := exec.Command(boshBinaryPath, []string{"ssh", "dns/0", "-c", "sudo mv /tmp/resolv.conf.backup `readlink /etc/resolv.conf`"}...)
				session, err := gexec.Start(restore, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session, 10*time.Second).Should(gexec.Exit(0))
			})

			It("rewrites the nameserver configuration back to our dns server", func() {
				junkResolvConf := exec.Command(boshBinaryPath, []string{"ssh", "dns/0", "-c", "echo 'nameserver 192.0.2.100' | sudo tee `realpath /etc/resolv.conf` > /dev/null"}...)
				session, err := gexec.Start(junkResolvConf, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session, 10*time.Second).Should(gexec.Exit(0))

				Eventually(func() *gexec.Session {
					cmd := exec.Command(boshBinaryPath, []string{"ssh", "dns/0", "-c", "dig +time=3 +tries=1 -t A healthcheck.bosh-dns."}...)
					session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Eventually(session, 10*time.Second).Should(gexec.Exit())

					return session
				}, 20*time.Second, time.Second*2).Should(gexec.Exit(0))

				output := string(session.Out.Contents())
				Expect(output).To(ContainSubstring(";; flags: qr aa rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
				Expect(output).To(MatchRegexp("healthcheck\\.bosh-dns\\.\\s+0\\s+IN\\s+A\\s+127\\.0\\.0\\.1"))
			})
		})
	})
})
