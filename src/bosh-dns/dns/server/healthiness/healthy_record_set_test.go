package healthiness_test

import (
	"bosh-dns/dns/server/healthiness"
	"bosh-dns/dns/server/healthiness/healthinessfakes"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("HealthyRecordSet", func() {
	var (
		fakeRecordSet     *healthinessfakes.FakeRecordSet
		fakeHealthWatcher *healthinessfakes.FakeHealthWatcher
		subscriptionChan  chan bool
		shutdownChan      chan struct{}

		recordSet *healthiness.HealthyRecordSet
	)

	BeforeEach(func() {
		fakeRecordSet = &healthinessfakes.FakeRecordSet{}
		fakeHealthWatcher = &healthinessfakes.FakeHealthWatcher{}
		subscriptionChan = make(chan bool)
		fakeRecordSet.SubscribeReturns(subscriptionChan)
		shutdownChan = make(chan struct{})

		fakeRecordSet.ResolveReturns([]string{"123.123.123.123", "123.123.123.246"}, nil)
		recordSet = healthiness.NewHealthyRecordSet(fakeRecordSet, fakeHealthWatcher, 5, shutdownChan)
	})

	AfterEach(func() {
		if subscriptionChan != nil {
			close(subscriptionChan)
		}
		close(shutdownChan)
	})

	It("fails when passing in a bad domain", func() {
		fakeRecordSet.ResolveReturns(nil, errors.New("no resolvy"))
		_, _, err := recordSet.Resolve("q-%%%")
		Expect(err).To(HaveOccurred())
	})

	Context("when some ips are healthy", func() {
		BeforeEach(func() {
			fakeHealthWatcher.IsHealthyStub = func(ip string) bool {
				switch ip {
				case "123.123.123.123":
					return true
				case "123.123.123.246":
					return false
				}
				return false
			}
		})

		It("returns only the healthy ips", func() {
			ips, healthy, err := recordSet.Resolve("i.g.n.d.d.")
			Expect(err).NotTo(HaveOccurred())
			Expect(healthy).To(BeTrue())
			Expect(ips).To(ConsistOf("123.123.123.123"))
		})
	})

	Context("when all ips are un-healthy", func() {
		BeforeEach(func() {
			fakeHealthWatcher.IsHealthyReturns(false)
		})

		It("returns all ips", func() {
			ips, healthy, err := recordSet.Resolve("i.g.n.d.d.")
			Expect(err).NotTo(HaveOccurred())
			Expect(healthy).To(BeFalse())
			Expect(ips).To(ConsistOf("123.123.123.123", "123.123.123.246"))
		})
	})

	Context("when the ips under a tracked domain change", func() {
		BeforeEach(func() {
			recordSet.Resolve("i.g.n.d.d.")
			fakeRecordSet.ResolveReturns([]string{"123.123.123.123", "123.123.123.5"}, nil)

			Expect(fakeHealthWatcher.IsHealthyCallCount()).To(Equal(2))
			subscriptionChan <- true
			Eventually(fakeRecordSet.ResolveCallCount).Should(Equal(2))
		})

		It("returns the new ones", func() {
			ips, _, err := recordSet.Resolve("i.g.n.d.d.")
			Expect(err).NotTo(HaveOccurred())
			Expect(ips).To(ConsistOf("123.123.123.123", "123.123.123.5"))
		})

		It("checks the health of new ones", func() {
			Eventually(fakeHealthWatcher.IsHealthyCallCount).Should(Equal(3))
			Expect(fakeHealthWatcher.IsHealthyArgsForCall(2)).To(Equal("123.123.123.5"))
		})

		It("stops tracking old ones", func() {
			Eventually(fakeHealthWatcher.UntrackCallCount).Should(Equal(1))
			Expect(fakeHealthWatcher.UntrackArgsForCall(0)).To(Equal("123.123.123.246"))
		})
	})

	Context("when the ips not under a tracked domain change", func() {
		BeforeEach(func() {
			fakeRecordSet.ResolveReturns([]string{"123.123.123.123", "123.123.123.5"}, nil)

			Expect(fakeHealthWatcher.IsHealthyCallCount()).To(Equal(0))
			subscriptionChan <- true
		})

		It("returns the new ones", func() {
			ips, _, err := recordSet.Resolve("i.g.n.d.d.")
			Expect(err).NotTo(HaveOccurred())
			Expect(ips).To(ConsistOf("123.123.123.123", "123.123.123.5"))
		})

		It("does not checks the health of new ones", func() {
			Expect(fakeHealthWatcher.IsHealthyCallCount()).To(Equal(0))
		})

		It("doesn't untrack anything", func() {
			Expect(fakeHealthWatcher.UntrackCallCount()).To(Equal(0))
		})
	})

	Describe("limiting tracked domains", func() {
		BeforeEach(func() {
			fakeRecordSet.ResolveStub = func(domain string) ([]string, error) {
				for i := 0; i < 10; i++ {
					if domain == fmt.Sprintf("i%d.g.n.d.d.", i) {
						return []string{fmt.Sprintf("%d.%d.%d.%d", i, i, i, i)}, nil
					}
				}
				return nil, errors.New("NXDOMAIN")
			}

			subscriptionChan <- true
		})

		It("tracks no more than the maximum number of domains (5) domains", func() {
			for i := 0; i < 10; i++ {
				recordSet.Resolve(fmt.Sprintf("i%d.g.n.d.d.", i))
			}

			Eventually(fakeHealthWatcher.UntrackCallCount).Should(Equal(5))

			Expect([]string{
				fakeHealthWatcher.UntrackArgsForCall(0),
				fakeHealthWatcher.UntrackArgsForCall(1),
				fakeHealthWatcher.UntrackArgsForCall(2),
				fakeHealthWatcher.UntrackArgsForCall(3),
				fakeHealthWatcher.UntrackArgsForCall(4),
			}).To(ConsistOf(
				"0.0.0.0",
				"1.1.1.1",
				"2.2.2.2",
				"3.3.3.3",
				"4.4.4.4",
			))
		})
	})
})
