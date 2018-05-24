package healthiness_test

import (
	"bosh-dns/dns/server/healthiness"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("NopHealthWatcher", func() {
	var (
		signal  chan struct{}
		stopped chan struct{}

		healthWatcher healthiness.HealthWatcher
	)

	BeforeEach(func() {
		healthWatcher = healthiness.NewNopHealthWatcher()
		signal = make(chan struct{})
		stopped = make(chan struct{})

		go func() {
			healthWatcher.Run(signal)
			close(stopped)
		}()
	})

	AfterEach(func() {
		close(signal)
		Eventually(stopped).Should(BeClosed())
	})

	Describe("IsHealthy", func() {
		var ip string

		BeforeEach(func() {
			ip = "127.0.0.1"
		})

		It("is always healthy", func() {
			Expect(*healthWatcher.IsHealthy(ip)).To(BeTrue())
		})
	})
})
