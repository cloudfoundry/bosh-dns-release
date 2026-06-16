//go:build linux || darwin

package linux_test

import (
	"errors"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"bosh-dns/acceptance_tests/helpers"

	. "github.com/onsi/ginkgo/v2"
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
		cmd := exec.Command(boshBinaryPath, []string{"ssh", firstInstanceSlug, "-c", "dig +tcp upcheck.bosh-dns. @169.254.0.2"}...)
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session, 10*time.Second).Should(gexec.Exit(0))
		Expect(session.Out).Should(gbytes.Say("Got answer:"))
		Expect(session.Out).Should(gbytes.Say("flags: qr aa rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
		Expect(session.Out).Should(gbytes.Say("upcheck\\.bosh-dns\\.\\s+0\\s+IN\\s+A\\s+127\\.0\\.0\\.1"))
		Expect(session.Out).Should(gbytes.Say("SERVER: 169.254.0.2#53"))
	})

	It("exposes a debug API through a CLI", func() {
		// pipe to cat to remove color codes
		cmd := exec.Command(boshBinaryPath, []string{"ssh", firstInstanceSlug, "-c", "/var/vcap/jobs/bosh-dns/bin/cli instances | cat"}...)
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session, 10*time.Second).Should(gexec.Exit(0))

		Expect(session.Out).To(gbytes.Say(`ID\s+Group\s+Network\s+Deployment\s+IP\s+Domain\s+AZ\s+Index\s+HealthState`))
		Expect(session.Out).To(gbytes.Say(`[a-z0-9\-]{36}\s+bosh-dns\s+default\s+bosh-dns\s+[0-9.]+\s+bosh\.\s+z1\s+0\s+[a-z]+`))
	})

	It("should respond to udp dns queries", func() {
		cmd := exec.Command(boshBinaryPath, []string{"ssh", firstInstanceSlug, "-c", "dig +notcp upcheck.bosh-dns. @169.254.0.2"}...)
		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session, 10*time.Second).Should(gexec.Exit(0))
		Expect(session.Out).Should(gbytes.Say("Got answer:"))
		Expect(session.Out).Should(gbytes.Say("flags: qr aa rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
		Expect(session.Out).Should(gbytes.Say(";upcheck\\.bosh-dns\\.\\s+IN\\s+A"))
		Expect(session.Out).Should(gbytes.Say("SERVER: 169.254.0.2#53"))
	})

	Context("when the healtcheck becomes unreachable", func() {
		AfterEach(func() {
			cmd := exec.Command(boshBinaryPath, []string{"ssh", firstInstanceSlug, "-c", "sudo /sbin/iptables -D INPUT -p udp -j DROP"}...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session, 10*time.Second).Should(gexec.Exit())
		})

		It("should kill itself if its upcheck becomes unreachable", func() {
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

			newServerPid, err := getServerPid()
			if err == nil {
				// the DNS server flaps in this condition, so it is possible
				//   our lsof occurred during its brief uptime and thus exited 0
				Expect(newServerPid).NotTo(Equal(originalServerPid))
			}

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
		It("resolves the bosh-dns upcheck", func() {
			cmd := exec.Command(boshBinaryPath, []string{"ssh", "bosh-dns/0", "-c", "dig -t A upcheck.bosh-dns."}...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session, 10*time.Second).Should(gexec.Exit(0))
			output := string(session.Out.Contents())
			Expect(output).To(ContainSubstring(";; flags: qr aa rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
		})

		Context("when configure_systemd_resolved is enabled (noble stemcell)", func() {
			// The bosh-dns dummy interface is only created when configure_systemd_resolved=true,
			// which is set exclusively for ubuntu-noble in the bosh-dns-systemd runtime config addon.
			// On jammy and earlier, bosh-dns binds to a loopback alias instead — no bosh-dns link exists.
			hasBoshDnsInterface := func() bool {
				cmd := exec.Command(boshBinaryPath, []string{"ssh", firstInstanceSlug, "-c",
					"ip link show bosh-dns > /dev/null 2>&1 && echo yes || echo no"}...)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session, 10*time.Second).Should(gexec.Exit(0))
				return strings.Contains(string(session.Out.Contents()), "yes")
			}

			It("does not set the bosh-dns interface as the default DNS route", func() {
				// bosh-dns must never hold +DefaultRoute on the bosh-dns dummy interface.
				// If it does, all external DNS queries are routed to bosh-dns instead of
				// the IaaS-provided upstream - causing REFUSED on warden containers where
				// no physical NIC has DHCP DNS to serve as an alternative default route.
				if !hasBoshDnsInterface() {
					Skip("bosh-dns dummy interface not present — configure_systemd_resolved not enabled on this stemcell")
				}

				cmd := exec.Command(boshBinaryPath, []string{"ssh", firstInstanceSlug, "-c",
					"resolvectl status bosh-dns 2>/dev/null | grep Protocols"}...)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session, 10*time.Second).Should(gexec.Exit(0))
				Expect(session.Out).To(gbytes.Say(`-DefaultRoute`))
			})

			It("resolves external names via the OS resolver when disable_recursors is true", func() {
				// Verify that external DNS works end-to-end via systemd-resolved's
				// upstream (IaaS DHCP DNS), not through bosh-dns. Regression test for
				// the warden noble DNS issue where bosh-dns incorrectly held +DefaultRoute
				// and intercepted all external queries, returning REFUSED.
				//
				// The regression only manifests when disable_recursors=true — if it is
				// false, bosh-dns forwards external queries anyway and the test would pass
				// even with a broken +DefaultRoute. Guard on the rendered config first.
				if !hasBoshDnsInterface() {
					Skip("bosh-dns dummy interface not present — configure_systemd_resolved not enabled on this stemcell")
				}

				// Read disable_recursors from the rendered config.json on the VM.
				cmd := exec.Command(boshBinaryPath, []string{"ssh", firstInstanceSlug, "-c",
					`python3 -c "import json; c=json.load(open('/var/vcap/jobs/bosh-dns/config/config.json')); print(c.get('disable_recursors', False))" 2>/dev/null`}...)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session, 10*time.Second).Should(gexec.Exit(0))
				if !strings.Contains(string(session.Out.Contents()), "True") {
					Skip("disable_recursors is not true on this deployment — test only validates warden regression when disable_recursors=true")
				}

				// Use dig for stable output — check status: NOERROR rather than
				// nslookup's locale-sensitive "Non-authoritative answer" string.
				cmd = exec.Command(boshBinaryPath, []string{"ssh", firstInstanceSlug, "-c",
					"dig +time=5 google.com 2>&1"}...)
				session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session, 15*time.Second).Should(gexec.Exit(0))
				Expect(session.Out).To(gbytes.Say(`status: NOERROR`))
			})
		})

		Context("external processes changing /etc/resolv.conf", func() {
			BeforeEach(func() {
				// TODO: remove when Jammy goes EOL
				if !helpers.OverrideNameserverFor(baseStemcell) {
					Skip("bosh-dns-resolvconf is disabled on non-Jammy stemcells; /etc/resolv.conf is managed by systemd-resolved")
				}

				backup := exec.Command(boshBinaryPath, []string{"ssh", "bosh-dns/0", "-c", "sudo cp `readlink /etc/resolv.conf` /tmp/resolv.conf.backup"}...)

				session, err := gexec.Start(backup, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session, 10*time.Second).Should(gexec.Exit(0))
			})

			AfterEach(func() {
				// TODO: remove when Jammy goes EOL
				if !helpers.OverrideNameserverFor(baseStemcell) {
					return
				}
				restore := exec.Command(boshBinaryPath, []string{"ssh", "bosh-dns/0", "-c", "sudo mv /tmp/resolv.conf.backup `readlink /etc/resolv.conf`"}...)
				session, err := gexec.Start(restore, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session, 10*time.Second).Should(gexec.Exit(0))
			})

			It("rewrites the nameserver configuration back to our dns server", func() {
				junkResolvConf := exec.Command(boshBinaryPath, []string{"ssh", "bosh-dns/0", "-c", "echo 'nameserver 192.0.2.100' | sudo tee `realpath /etc/resolv.conf` > /dev/null"}...)
				session, err := gexec.Start(junkResolvConf, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session, 10*time.Second).Should(gexec.Exit(0))

				Eventually(func() *gexec.Session {
					cmd := exec.Command(boshBinaryPath, []string{"ssh", "bosh-dns/0", "-c", "dig +time=3 +tries=1 -t A upcheck.bosh-dns."}...)
					session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Eventually(session, 10*time.Second).Should(gexec.Exit())

					return session
				}, 20*time.Second, time.Second*1).Should(gexec.Exit(0))

				output := string(session.Out.Contents())
				Expect(output).To(ContainSubstring(";; flags: qr aa rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
				Expect(output).To(MatchRegexp("upcheck\\.bosh-dns\\.\\s+0\\s+IN\\s+A\\s+127\\.0\\.0\\.1"))
			})
		})
	})
})
