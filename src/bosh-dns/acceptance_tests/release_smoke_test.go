package acceptance

import (
	"bosh-dns/acceptance_tests/helpers"
	"fmt"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"os/exec"

	"time"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Integration", func() {
	var firstInstance helpers.InstanceInfo

	Describe("DNS endpoint", func() {
		BeforeEach(func() {
			ensureRecursorIsDefinedByDnsRelease()
			firstInstance = allDeployedInstances[0]
		})

		It("returns records for bosh instances", func() {
			cmd := exec.Command("dig", strings.Split(fmt.Sprintf("-t A %s.bosh-dns.default.bosh-dns.bosh @%s", firstInstance.InstanceID, firstInstance.IP), " ")...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))

			Eventually(session.Out).Should(gbytes.Say("Got answer:"))
			Eventually(session.Out).Should(gbytes.Say("flags: qr aa rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
			Eventually(session.Out).Should(gbytes.Say(
				"%s\\.bosh-dns\\.default\\.bosh-dns\\.bosh\\.\\s+0\\s+IN\\s+A\\s+%s",
				firstInstance.InstanceID,
				firstInstance.IP))
			Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("SERVER: %s#53", firstInstance.IP)))
		})

		It("returns Rcode failure for arpaing bosh instances", func() {
			cmd := exec.Command("dig", strings.Split(fmt.Sprintf("-x %s @%s", firstInstance.IP, firstInstance.IP), " ")...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			<-session.Exited
			Expect(session.ExitCode()).To(BeZero())

			Eventually(session.Out).Should(gbytes.Say("Got answer:"))
			Eventually(session.Out).Should(gbytes.Say(`;; ->>HEADER<<- opcode: QUERY, status: SERVFAIL, id: \d+`))
			Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("SERVER: %s#53", firstInstance.IP)))
		})

		It("returns records for bosh instances found with query for all records", func() {
			Expect(len(allDeployedInstances)).To(BeNumerically(">", 1))

			cmd := exec.Command("dig", strings.Split(fmt.Sprintf("-t A q-s0.bosh-dns.default.bosh-dns.bosh @%s", firstInstance.IP), " ")...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			<-session.Exited
			Expect(session.ExitCode()).To(BeZero())

			output := string(session.Out.Contents())
			Expect(output).To(ContainSubstring("Got answer:"))
			Expect(output).To(ContainSubstring("flags: qr aa rd ra; QUERY: 1, ANSWER: %d, AUTHORITY: 0, ADDITIONAL: 0", len(allDeployedInstances)))
			for _, info := range allDeployedInstances {
				Expect(output).To(MatchRegexp("q-s0\\.bosh-dns\\.default\\.bosh-dns\\.bosh\\.\\s+0\\s+IN\\s+A\\s+%s", info.IP))
			}
			Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("SERVER: %s#53", firstInstance.IP)))
		})

		It("returns records for bosh instances found with query for index", func() {
			Expect(len(allDeployedInstances)).To(BeNumerically(">", 1))

			cmd := exec.Command("dig", strings.Split(fmt.Sprintf("-t A q-i%s.bosh-dns.default.bosh-dns.bosh @%s", firstInstance.Index, firstInstance.IP), " ")...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			<-session.Exited
			Expect(session.ExitCode()).To(BeZero())

			output := string(session.Out.Contents())
			Expect(output).To(ContainSubstring("Got answer:"))
			Expect(output).To(ContainSubstring("flags: qr aa rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
			Expect(output).To(MatchRegexp("q-i%s\\.bosh-dns\\.default\\.bosh-dns\\.bosh\\.\\s+0\\s+IN\\s+A\\s+%s", firstInstance.Index, firstInstance.IP))
			Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("SERVER: %s#53", firstInstance.IP)))
		})

		It("finds and resolves aliases specified in other jobs on the same instance", func() {
			cmd := exec.Command("dig", strings.Split(fmt.Sprintf("-t A internal.alias. @%s", firstInstance.IP), " ")...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			<-session.Exited
			Expect(session.ExitCode()).To(BeZero())

			Eventually(session.Out).Should(gbytes.Say("Got answer:"))
			Eventually(session.Out).Should(gbytes.Say("flags: qr aa rd ra; QUERY: 1, ANSWER: %d, AUTHORITY: 0, ADDITIONAL: 0", len(allDeployedInstances)))

			Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("SERVER: %s#53", firstInstance.IP)))
		})

		It("resolves alias globs", func() {
			for _, alias := range []string{"asterisk.alias.", "another.asterisk.alias.", "yetanother.asterisk.alias."} {
				cmdArgs := fmt.Sprintf("-t A %s @%s", alias, firstInstance.IP)
				cmd := exec.Command("dig", strings.Split(cmdArgs, " ")...)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-session.Exited
				Expect(session.ExitCode()).To(BeZero())

				Eventually(session.Out).Should(gbytes.Say("Got answer:"))
				Eventually(session.Out).Should(gbytes.Say("flags: qr aa rd ra; QUERY: 1, ANSWER: %d, AUTHORITY: 0, ADDITIONAL: 0", len(allDeployedInstances)))

				Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("SERVER: %s#53", firstInstance.IP)))
			}
		})

		It("should resolve specified upcheck", func() {
			cmd := exec.Command("dig", strings.Split(fmt.Sprintf("-t A upcheck.bosh-dns. @%s", firstInstance.IP), " ")...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			<-session.Exited
			Expect(session.ExitCode()).To(BeZero())

			Eventually(session.Out).Should(gbytes.Say("Got answer:"))
			Eventually(session.Out).Should(gbytes.Say("flags: qr aa rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))

			Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("SERVER: %s#53", firstInstance.IP)))
		})
	})

	Context("Instance health", func() {
		var (
			osSuffix string
		)

		BeforeEach(func() {
			osSuffix = ""
			if testTargetOS == "windows" {
				osSuffix = "-windows"
			}
			ensureHealthEndpointDeployed("-o", assetPath("ops/enable-stop-a-job"+osSuffix+".yml"))
			firstInstance = allDeployedInstances[0]
		})

		It("returns a healthy response when the instance is running", func() {
			client := setupSecureGet()

			Eventually(func() string {
				return secureGetRespBody(client, firstInstance.IP, 2345).State
			}, 31*time.Second).Should(Equal("running"))
		})

		It("stops returning IP addresses of instances whose status becomes unknown", func() {
			var output string
			Expect(len(allDeployedInstances)).To(BeNumerically(">", 1))

			cmd := exec.Command("dig", strings.Split(fmt.Sprintf("-t A q-s0.bosh-dns.default.bosh-dns.bosh @%s", firstInstance.IP), " ")...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))

			output = string(session.Out.Contents())
			Expect(output).To(ContainSubstring("Got answer:"))
			Expect(output).To(ContainSubstring("flags: qr aa rd ra; QUERY: 1, ANSWER: %d, AUTHORITY: 0, ADDITIONAL: 0", len(allDeployedInstances)))
			for _, info := range allDeployedInstances {
				Expect(output).To(MatchRegexp("q-s0\\.bosh-dns\\.default\\.bosh-dns\\.bosh\\.\\s+0\\s+IN\\s+A\\s+%s", info.IP))
			}
			Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("SERVER: %s#53", firstInstance.IP)))

			secondInstanceSlug := fmt.Sprintf("%s/%s", allDeployedInstances[1].InstanceGroup, allDeployedInstances[1].InstanceID)
			helpers.Bosh("stop", secondInstanceSlug)

			defer func() {
				helpers.Bosh("start", secondInstanceSlug)

				Eventually(func() string {
					cmd := exec.Command("dig", strings.Split(fmt.Sprintf("-t A q-s0.bosh-dns.default.bosh-dns.bosh @%s", firstInstance.IP), " ")...)
					session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(session).Should(gexec.Exit(0))
					Expect(session.ExitCode()).To(BeZero())

					output = string(session.Out.Contents())

					return output
				}, 60*time.Second, 1*time.Second).Should(
					ContainSubstring("flags: qr aa rd ra; QUERY: 1, ANSWER: %d, AUTHORITY: 0, ADDITIONAL: 0", len(allDeployedInstances)),
				)

				Expect(output).To(ContainSubstring("Got answer:"))
				for _, info := range allDeployedInstances {
					Expect(output).To(MatchRegexp("q-s0\\.bosh-dns\\.default\\.bosh-dns\\.bosh\\.\\s+0\\s+IN\\s+A\\s+%s", info.IP))
				}
			}()

			Eventually(func() string {
				cmd := exec.Command("dig", strings.Split(fmt.Sprintf("-t A q-s0.bosh-dns.default.bosh-dns.bosh @%s", firstInstance.IP), " ")...)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(session).Should(gexec.Exit(0))
				Expect(session.ExitCode()).To(BeZero())

				output = string(session.Out.Contents())

				return output
			}, 60*time.Second, 1*time.Second).Should(SatisfyAll(
				ContainSubstring("flags: qr aa rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"),
				MatchRegexp("q-s0\\.bosh-dns\\.default\\.bosh-dns\\.bosh\\.\\s+0\\s+IN\\s+A\\s+%s", firstInstance.IP),
				Not(MatchRegexp("q-s0\\.bosh-dns\\.default\\.bosh-dns\\.bosh\\.\\s+0\\s+IN\\s+A\\s+%s", allDeployedInstances[1].IP)),
			))
			// ^ timeout = agent heartbeat updates health.json every 20s + dns checks healthiness every 20s + a buffer interval
		})

		It("stops returning IP addresses of instances that become unhealthy", func() {
			var output string
			Expect(len(allDeployedInstances)).To(BeNumerically(">", 1))

			cmd := exec.Command("dig", strings.Split(fmt.Sprintf("-t A q-s0.bosh-dns.default.bosh-dns.bosh @%s", firstInstance.IP), " ")...)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			<-session.Exited
			Expect(session.ExitCode()).To(BeZero())

			output = string(session.Out.Contents())
			Expect(output).To(ContainSubstring("Got answer:"))
			Expect(output).To(ContainSubstring("flags: qr aa rd ra; QUERY: 1, ANSWER: %d, AUTHORITY: 0, ADDITIONAL: 0", len(allDeployedInstances)))
			for _, info := range allDeployedInstances {
				Expect(output).To(MatchRegexp("q-s0\\.bosh-dns\\.default\\.bosh-dns\\.bosh\\.\\s+0\\s+IN\\s+A\\s+%s", info.IP))
			}
			Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("SERVER: %s#53", firstInstance.IP)))

			instanceSlug := fmt.Sprintf("%s/%s", allDeployedInstances[1].InstanceGroup, allDeployedInstances[1].InstanceID)
			helpers.BoshRunErrand("stop-a-job"+osSuffix, instanceSlug)

			defer func() {
				helpers.Bosh("start", instanceSlug)
			}()

			Eventually(func() string {
				cmd := exec.Command("dig", strings.Split(fmt.Sprintf("-t A q-s0.bosh-dns.default.bosh-dns.bosh @%s", firstInstance.IP), " ")...)
				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-session.Exited
				Expect(session.ExitCode()).To(BeZero())

				output = string(session.Out.Contents())

				return output
			}, 60*time.Second, 1*time.Second).Should(ContainSubstring("flags: qr aa rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0"))
			// ^ timeout = agent heartbeat updates health.json every 20s + dns checks healthiness every 20s + a buffer interval

			Expect(output).To(MatchRegexp("q-s0\\.bosh-dns\\.default\\.bosh-dns\\.bosh\\.\\s+0\\s+IN\\s+A\\s+%s", firstInstance.IP))
			Expect(output).ToNot(MatchRegexp("q-s0\\.bosh-dns\\.default\\.bosh-dns\\.bosh\\.\\s+0\\s+IN\\s+A\\s+%s", allDeployedInstances[1].IP))
		})

		Context("when a job defines a healthy executable", func() {
			var (
				osSuffix string
			)

			BeforeEach(func() {
				osSuffix = ""
				if testTargetOS == "windows" {
					osSuffix = "-windows"
				}
				ensureHealthEndpointDeployed("-o", assetPath("ops/enable-healthy-executable-job"+osSuffix+".yml"))
			})

			It("changes the health endpoint return value based on how the executable exits", func() {
				client := setupSecureGet()
				lastInstance := allDeployedInstances[1]
				lastInstanceSlug := fmt.Sprintf("%s/%s", lastInstance.InstanceGroup, lastInstance.InstanceID)

				Eventually(func() string {
					return secureGetRespBody(client, lastInstance.IP, 2345).State
				}, 31*time.Second).Should(Equal("running"))

				helpers.BoshRunErrand("make-health-executable-job-unhealthy"+osSuffix, lastInstanceSlug)

				Eventually(func() string {
					return secureGetRespBody(client, lastInstance.IP, 2345).State
				}, 31*time.Second).Should(Equal("failing"))

				helpers.BoshRunErrand("make-health-executable-job-healthy"+osSuffix, lastInstanceSlug)

				Eventually(func() string {
					return secureGetRespBody(client, lastInstance.IP, 2345).State
				}, 31*time.Second).Should(Equal("running"))
			})
		})
	})

	Describe("link dns names", func() {
		var (
			osSuffix string
		)

		BeforeEach(func() {
			osSuffix = ""
			if testTargetOS == "windows" {
				osSuffix = "-windows"
			}
			ensureHealthEndpointDeployed(
				"-o", assetPath("ops/enable-link-dns-addresses.yml"),
				"-o", assetPath("ops/enable-healthy-executable-job"+osSuffix+".yml"),
			)
			firstInstance = allDeployedInstances[0]
		})

		It("respects health status according to job providing link", func() {
			client := setupSecureGet()
			lastInstance := allDeployedInstances[1]
			lastInstanceSlug := fmt.Sprintf("%s/%s", lastInstance.InstanceGroup, lastInstance.InstanceID)
			output := helpers.BoshRunErrand("get-healthy-executable-linked-address"+osSuffix, lastInstanceSlug)
			address := strings.TrimSpace(strings.Split(strings.Split(output, "ADDRESS:")[1], "\n")[0])
			Expect(address).To(MatchRegexp(`^q-n\d+s0\.q-g\d+\.bosh$`))
			re := regexp.MustCompile(`^q-n\d+s0\.q-g(\d+)\.bosh$`)
			groupID := re.FindStringSubmatch(address)[1]

			Eventually(func() string {
				return secureGetRespBody(client, lastInstance.IP, 2345).GroupState[groupID]
			}, 31*time.Second).Should(Equal("running"))

			Eventually(func() []string {
				return resolve(address, firstInstance.IP)
			}, 31*time.Second).Should(ConsistOf(firstInstance.IP, lastInstance.IP))

			helpers.BoshRunErrand("make-health-executable-job-unhealthy"+osSuffix, lastInstanceSlug)

			Eventually(func() string {
				return secureGetRespBody(client, lastInstance.IP, 2345).GroupState[groupID]
			}, 31*time.Second).Should(Equal("failing"))

			Eventually(func() []string {
				return resolve(address, firstInstance.IP)
			}, 31*time.Second).Should(ConsistOf(firstInstance.IP))

			helpers.BoshRunErrand("make-health-executable-job-healthy"+osSuffix, lastInstanceSlug)

			Eventually(func() string {
				return secureGetRespBody(client, lastInstance.IP, 2345).GroupState[groupID]
			}, 31*time.Second).Should(Equal("running"))

			Eventually(func() []string {
				return resolve(address, firstInstance.IP)
			}, 31*time.Second).Should(ConsistOf(firstInstance.IP, lastInstance.IP))
		})
	})
})
