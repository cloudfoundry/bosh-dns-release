package healthiness_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"bosh-dns/dns/server/healthiness"
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
})
