package handlers_test

import (
	. "bosh-dns/dns/server/handlers"

	"errors"
	"time"

	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RecursorPool", func() {
	var (
		pool                 RecursorPool
		work                 func(string) error
		recursorsFailOncePer [3]int
		recursorAttempts     [3]int
		fakeLogger           *loggerfakes.FakeLogger
	)

	BeforeEach(func() {
		recursorsFailOncePer = [3]int{1000, 1000, 1000}
		recursorAttempts = [3]int{0, 0, 0}
		fakeLogger = &loggerfakes.FakeLogger{}
	})

	JustBeforeEach(func() {
		recursors := []string{
			"one",
			"two",
			"three",
		}

		workFuncs := [3]func() error{}

		for i := 0; i < 3; i++ {
			index := i
			workFuncs[index] = func() error {
				recursorAttempts[index] = recursorAttempts[index] + 1
				if recursorAttempts[index]%recursorsFailOncePer[index] == 0 {
					return errors.New("flaked out!")
				} else {
					return nil
				}
			}
		}

		work = func(recursor string) error {
			switch recursor {
			case "one":
				return workFuncs[0]()
			case "two":
				return workFuncs[1]()
			case "three":
				return workFuncs[2]()
			default:
				return errors.New("that's not real")
			}
		}

		pool = NewFailoverRecursorPool(recursors, fakeLogger)
	})

	It("returns an error if there are no recursors configured", func() {
		pool = NewFailoverRecursorPool([]string{}, fakeLogger)
		Expect(pool.PerformStrategically(func(string) error { return nil })).To(HaveOccurred())

		pool = NewFailoverRecursorPool(nil, fakeLogger)
		Expect(pool.PerformStrategically(func(string) error { return nil })).To(HaveOccurred())
	})

	It("performs the requested work using first recursor by default", func() {
		for time := 0; time < 10; time++ {
			err := pool.PerformStrategically(work)
			Expect(err).ToNot(HaveOccurred())
		}

		Expect(fakeLogger.InfoCallCount()).To(Equal(1))
		tag, logMsg, _ := fakeLogger.InfoArgsForCall(0)
		Expect(logMsg).To(ContainSubstring("starting preference: one\n"))
		Expect(tag).To(Equal("FailoverRecursor"))

		Expect(recursorAttempts[0]).To(Equal(10))
	})

	Context("when the preferred recursor is occasionally flaky", func() {
		BeforeEach(func() {
			recursorsFailOncePer[0] = 6
		})

		It("will tolerate a few sparse failures and failover without changing preference", func() {
			for time := 0; time < 30; time++ {
				pool.PerformStrategically(work)
			}

			Expect(recursorAttempts[0]).To(Equal(30))
			Expect(recursorAttempts[1]).To(Equal(5))
		})
	})

	Context("when the first N recursors fail five times in a short period", func() {
		BeforeEach(func() {
			recursorsFailOncePer[0] = 3
			recursorsFailOncePer[1] = 3
		})

		It("begins to prefer the N+1st recursor", func() {
			for time := 0; time < 1000; time++ {
				pool.PerformStrategically(work)
			}

			Expect(fakeLogger.InfoCallCount()).To(Equal(3))
			_, logMsg, _ := fakeLogger.InfoArgsForCall(0)
			Expect(logMsg).To(ContainSubstring("starting preference: one\n"))
			_, logMsg, _ = fakeLogger.InfoArgsForCall(1)
			Expect(logMsg).To(ContainSubstring("shifting recursor preference: two\n"))
			_, logMsg, _ = fakeLogger.InfoArgsForCall(2)
			Expect(logMsg).To(ContainSubstring("shifting recursor preference: three\n"))

			Expect(recursorAttempts[0]).To(BeNumerically("<", recursorAttempts[2]))
			Expect(recursorAttempts[1]).To(BeNumerically("<", recursorAttempts[2]))
		})
	})

	Context("when the healthy recursor is the second one", func() {
		BeforeEach(func() {
			recursorsFailOncePer[0] = 3
			recursorsFailOncePer[2] = 3
		})

		It("settles on the second (health) recursor and does not fail over to the third", func() {
			for time := 0; time < 1000; time++ {
				pool.PerformStrategically(work)
			}

			Expect(fakeLogger.InfoCallCount()).To(Equal(2))
			_, logMsg, _ := fakeLogger.InfoArgsForCall(0)
			Expect(logMsg).To(ContainSubstring("starting preference: one\n"))
			_, logMsg, _ = fakeLogger.InfoArgsForCall(1)
			Expect(logMsg).To(ContainSubstring("shifting recursor preference: two\n"))

			Expect(recursorAttempts[0]).To(BeNumerically("<", recursorAttempts[1]))
			Expect(recursorAttempts[2]).To(Equal(0))
		})
	})

	It("can handle concurrent tries", func() {
		smash := func(done chan struct{}) {
			defer func() { done <- struct{}{} }()
			for i := 0; i < 15; i++ {
				pool.PerformStrategically(func(n string) error {
					if n == "one" {
						return errors.New("yikes")
					}
					return nil
				})
			}
		}

		done := make(chan struct{})

		go smash(done)
		go smash(done)

		for i := 0; i < 2; i++ {
			select {
			case <-done:
				continue
			case <-time.After(time.Minute):
				Fail("reached something like a deadlock")
			}
		}
	})
})
