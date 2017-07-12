package healthiness_test

import (
	"bosh-dns/dns/server/healthiness"
	"bosh-dns/dns/server/healthiness/healthinessfakes"
	"bosh-dns/dns/server/records"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("HealthyRecordSet", func() {
	var (
		fakeRecordSetRepo *healthinessfakes.FakeRecordSetRepo
		fakeHealthWatcher *healthinessfakes.FakeHealthWatcher
		innerRecordSet    records.RecordSet
		subscriptionChan  chan bool
		shutdownChan      chan struct{}

		recordSet *healthiness.HealthyRecordSet
	)

	BeforeEach(func() {
		fakeRecordSetRepo = &healthinessfakes.FakeRecordSetRepo{}
		fakeHealthWatcher = &healthinessfakes.FakeHealthWatcher{}
		subscriptionChan = make(chan bool)
		fakeRecordSetRepo.SubscribeReturns(subscriptionChan)
		shutdownChan = make(chan struct{})

		innerRecordSet = records.RecordSet{
			Records: []records.Record{
				{Id: "i", Group: "g", Network: "n", Deployment: "d", Ip: "123.123.123.123", Domain: "d."},
				{Id: "i", Group: "g", Network: "n", Deployment: "d", Ip: "123.123.123.246", Domain: "d."},
			},
		}
		fakeRecordSetRepo.GetReturns(innerRecordSet, nil)
		recordSet = healthiness.NewHealthyRecordSet(fakeRecordSetRepo, fakeHealthWatcher, shutdownChan)
	})

	AfterEach(func() {
		if subscriptionChan != nil {
			close(subscriptionChan)
		}
		close(shutdownChan)
	})

	It("fails when passing in a bad domain", func() {
		_, err := recordSet.Resolve("q-%%%")
		Expect(err).To(HaveOccurred())
	})

	Describe("refreshing record set", func() {
		It("does not refreshes the record set on every resolve", func() {
			for i := 0; i < 5; i++ {
				recordSet.Resolve("i.g.n.d.d.")
			}
			Expect(fakeRecordSetRepo.GetCallCount()).To(Equal(1))
		})

		It("refreshes the record when notified", func() {
			for i := 1; i <= 5; i++ {
				ip := fmt.Sprintf("123.123.123.%d", i)
				innerRecordSet = records.RecordSet{
					Records: []records.Record{
						{Id: "i", Group: "g", Network: "n", Deployment: "d", Ip: ip, Domain: "d."},
					},
				}
				fakeRecordSetRepo.GetReturns(innerRecordSet, nil)

				subscriptionChan <- true
				Eventually(fakeRecordSetRepo.GetCallCount).Should(Equal(1 + i))
				ips, err := recordSet.Resolve("i.g.n.d.d.")
				Expect(err).NotTo(HaveOccurred())
				Expect(ips).To(ConsistOf(ip))
			}
		})

		It("stops refreshing if the repo is closed", func() {
			close(subscriptionChan)
			subscriptionChan = nil
			Consistently(fakeRecordSetRepo.GetCallCount).Should(Equal(1))
		})

		Context("when refreshing the record set errors", func() {
			BeforeEach(func() {
				fakeRecordSetRepo.GetReturns(records.RecordSet{}, errors.New("could not fetch record set"))
			})

			It("keeps the old recordset", func() {
				subscriptionChan <- true
				Eventually(fakeRecordSetRepo.GetCallCount).Should(BeNumerically(">", 1))
				ips, err := recordSet.Resolve("i.g.n.d.d.")
				Expect(err).NotTo(HaveOccurred())
				Expect(ips).To(ConsistOf("123.123.123.123", "123.123.123.246"))
			})
		})

		Context("when all ips are healthy", func() {
			BeforeEach(func() {
				fakeHealthWatcher.IsHealthyReturns(true)
			})

			It("returns all ips", func() {
				ips, err := recordSet.Resolve("i.g.n.d.d.")
				Expect(err).NotTo(HaveOccurred())

				Expect(ips).To(ConsistOf("123.123.123.123", "123.123.123.246"))
			})
		})
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
			ips, err := recordSet.Resolve("i.g.n.d.d.")
			Expect(err).NotTo(HaveOccurred())

			Expect(ips).To(ConsistOf("123.123.123.123"))
		})
	})

	Context("when all ips are un-healthy", func() {
		BeforeEach(func() {
			fakeHealthWatcher.IsHealthyReturns(false)
		})

		It("returns all ips", func() {
			ips, err := recordSet.Resolve("i.g.n.d.d.")
			Expect(err).NotTo(HaveOccurred())

			Expect(ips).To(ConsistOf("123.123.123.123", "123.123.123.246"))
		})
	})

	Context("when the ips under a tracked domain change", func() {
		BeforeEach(func() {
			recordSet.Resolve("i.g.n.d.d.")
			innerRecordSet = records.RecordSet{
				Records: []records.Record{
					{Id: "i", Group: "g", Network: "n", Deployment: "d", Ip: "123.123.123.123", Domain: "d."},
					{Id: "i", Group: "g", Network: "n", Deployment: "d", Ip: "123.123.123.5", Domain: "d."},
				},
			}
			fakeRecordSetRepo.GetReturns(innerRecordSet, nil)

			Expect(fakeHealthWatcher.IsHealthyCallCount()).To(Equal(2))
			subscriptionChan <- true
			Eventually(fakeRecordSetRepo.GetCallCount).Should(Equal(2))
		})

		It("returns the new ones", func() {
			ips, err := recordSet.Resolve("i.g.n.d.d.")
			Expect(err).NotTo(HaveOccurred())
			Expect(ips).To(ConsistOf("123.123.123.123", "123.123.123.5"))
		})

		It("checks the health of new ones", func() {
			Expect(fakeHealthWatcher.IsHealthyCallCount()).To(Equal(3))

			// healthCheckedIPs := []string{
			// 	fakeHealthWatcher.IsHealthyArgsForCall(2),
			// 	fakeHealthWatcher.IsHealthyArgsForCall(3),
			// }
			// Expect(healthCheckedIPs).To(ConsistOf("123.123.123.123", "123.123.123.5"))

			Expect(fakeHealthWatcher.IsHealthyArgsForCall(2)).To(Equal("123.123.123.5"))
		})

		It("stops tracking old ones", func() {
			Eventually(fakeHealthWatcher.UntrackCallCount).Should(Equal(1))
			Expect(fakeHealthWatcher.UntrackArgsForCall(0)).To(Equal("123.123.123.246"))
		})
	})

	Context("when the ips not under a tracked domain change", func() {
		BeforeEach(func() {
			innerRecordSet = records.RecordSet{
				Records: []records.Record{
					{Id: "i", Group: "g", Network: "n", Deployment: "d", Ip: "123.123.123.123", Domain: "d."},
					{Id: "i", Group: "g", Network: "n", Deployment: "d", Ip: "123.123.123.5", Domain: "d."},
				},
			}
			fakeRecordSetRepo.GetReturns(innerRecordSet, nil)

			Expect(fakeHealthWatcher.IsHealthyCallCount()).To(Equal(0))
			subscriptionChan <- true
			Eventually(fakeRecordSetRepo.GetCallCount).Should(Equal(2))
		})

		It("returns the new ones", func() {
			ips, err := recordSet.Resolve("i.g.n.d.d.")
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
})
