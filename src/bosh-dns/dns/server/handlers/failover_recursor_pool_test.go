package handlers_test

import (
	"errors"
	"net"
	"sync"
	"time"

	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"bosh-dns/dns/config"
	"bosh-dns/dns/server"
	. "bosh-dns/dns/server/handlers"
)

type workFunc func(string) error

var _ = Describe("RecursorPool", func() {
	Context(`when recursor selection is "serial"`, func() {
		var (
			work func(map[string]int) workFunc
		)

		JustBeforeEach(func() {
			work = func(recursorCallCount map[string]int) workFunc {
				return func(recursor string) error {
					if _, ok := recursorCallCount[recursor]; ok {
						recursorCallCount[recursor]++
						return nil
					}

					return errors.New("not tracking recursor")
				}
			}
		})

		It("always starts at the beginning", func() {
			fakeLogger := &loggerfakes.FakeLogger{}
			recursorCallCount := map[string]int{
				"one":   0,
				"two":   0,
				"three": 0,
			}

			pool := NewFailoverRecursorPool([]string{"one", "two", "three"}, config.SerialRecursorSelection, 0, fakeLogger)
			err := pool.PerformStrategically(work(recursorCallCount))
			Expect(err).NotTo(HaveOccurred())
			Expect(recursorCallCount).To(Equal(map[string]int{
				"one":   1,
				"two":   0,
				"three": 0,
			}))

			pool = NewFailoverRecursorPool([]string{"bad", "two", "three"}, config.SerialRecursorSelection, 0, fakeLogger)
			err = pool.PerformStrategically(work(recursorCallCount))
			Expect(err).NotTo(HaveOccurred())
			Expect(recursorCallCount).To(Equal(map[string]int{
				"one":   1,
				"two":   1,
				"three": 0,
			}))

			pool = NewFailoverRecursorPool([]string{"bad", "bad", "three"}, config.SerialRecursorSelection, 0, fakeLogger)
			err = pool.PerformStrategically(work(recursorCallCount))
			Expect(err).NotTo(HaveOccurred())
			Expect(recursorCallCount).To(Equal(map[string]int{
				"one":   1,
				"two":   1,
				"three": 1,
			}))
		})

		Context("when all recursors fail", func() {
			var pool RecursorPool

			BeforeEach(func() {
				fakeLogger := &loggerfakes.FakeLogger{}
				pool = NewFailoverRecursorPool([]string{"bad", "bad", "bad"}, config.SerialRecursorSelection, 0, fakeLogger)
			})

			It("fails when all recursors fail", func() {
				Expect(pool.PerformStrategically(work(map[string]int{}))).To(Equal(ErrNoRecursorResponse))
			})
		})
	})

	Context(`when recursor selection is "smart"`, func() {
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

			pool = NewFailoverRecursorPool(recursors, config.SmartRecursorSelection, 0, fakeLogger)
		})

		It("returns an error if there are no recursors configured", func() {
			pool = NewFailoverRecursorPool([]string{}, config.SmartRecursorSelection, 0, fakeLogger)
			Expect(pool.PerformStrategically(func(string) error { return nil })).To(HaveOccurred())

			pool = NewFailoverRecursorPool(nil, config.SmartRecursorSelection, 0, fakeLogger)
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
					Expect(pool.PerformStrategically(work)).To(Succeed())
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
					Expect(pool.PerformStrategically(work)).To(Succeed())
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
					Expect(pool.PerformStrategically(work)).To(Succeed())
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
					pool.PerformStrategically(func(n string) error { //nolint:errcheck
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

	Context("perform with retry logic", func() {
		var (
			pool       RecursorPool
			fakeLogger = &loggerfakes.FakeLogger{}
			called     int
			netErr     *net.DNSError
		)
		BeforeEach(func() {
			called = 0
			netErr = &net.DNSError{
				Err:       "i/o timeout",
				IsTimeout: true,
			}
		})

		It("perform with retry logic with default configuration and valid error response", func() {
			pool = NewFailoverRecursorPool([]string{"retry"}, config.SmartRecursorSelection, 0, fakeLogger)
			err := pool.PerformStrategically(func(n string) error {
				called++
				return errors.New("NXDOMAIN")
			})

			Expect(called).To(Equal(1))
			Expect(err).To(HaveOccurred())
		})

		It("perform with retry logic with default configuration and network issue", func() {
			pool = NewFailoverRecursorPool([]string{"retry"}, config.SmartRecursorSelection, 0, fakeLogger)
			err := pool.PerformStrategically(func(n string) error {
				called++
				return netErr
			})

			Expect(called).To(Equal(1))
			Expect(err).To(HaveOccurred())
		})

		It("perform with retry logic with retry count 1", func() {
			pool = NewFailoverRecursorPool([]string{"retry"}, config.SmartRecursorSelection, 1, fakeLogger)
			err := pool.PerformStrategically(func(n string) error {
				called++
				return netErr
			})

			Expect(called).To(Equal(2))
			Expect(err).To(HaveOccurred())
		})

		It("perform with retry logic with retry count 3", func() {
			pool = NewFailoverRecursorPool([]string{"retry"}, config.SmartRecursorSelection, 3, fakeLogger)
			err := pool.PerformStrategically(func(n string) error {
				called++
				if called == 4 {
					return nil
				}
				return netErr
			})

			Expect(called).To(Equal(4))
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context(`when recursor selection is "race"`, func() {
		var (
			fakeLogger *loggerfakes.FakeLogger
		)

		BeforeEach(func() {
			fakeLogger = &loggerfakes.FakeLogger{}
		})

		It("returns an error if there are no recursors configured", func() {
			pool := NewFailoverRecursorPool([]string{}, config.RaceRecursorSelection, 0, fakeLogger)
			Expect(pool.PerformStrategically(func(string) error { return nil })).To(Equal(ErrNoRecursorResponse))

			pool = NewFailoverRecursorPool(nil, config.RaceRecursorSelection, 0, fakeLogger)
			Expect(pool.PerformStrategically(func(string) error { return nil })).To(Equal(ErrNoRecursorResponse))
		})

		It("queries all recursors in parallel", func() {
			callCount := make(map[string]int)
			var mu sync.Mutex
			started := make(chan string, 3)
			pool := NewFailoverRecursorPool([]string{"one", "two", "three"}, config.RaceRecursorSelection, 0, fakeLogger)

			err := pool.PerformStrategically(func(recursor string) error {
				started <- recursor // Signal that this recursor was called
				mu.Lock()
				callCount[recursor]++
				mu.Unlock()
				// Return an error so we don't return early - we want to verify all are called
				return errors.New("test error")
			})

			// All should have been started
			Expect(err).To(HaveOccurred())
			close(started)

			startedRecursors := make(map[string]bool)
			for r := range started {
				startedRecursors[r] = true
			}

			Expect(len(startedRecursors)).To(Equal(3))
			Expect(startedRecursors["one"]).To(BeTrue())
			Expect(startedRecursors["two"]).To(BeTrue())
			Expect(startedRecursors["three"]).To(BeTrue())
		})

		It("returns immediately on first NOERROR response", func() {
			completionOrder := make(chan string, 3)
			pool := NewFailoverRecursorPool([]string{"slow", "fast", "slower"}, config.RaceRecursorSelection, 0, fakeLogger)

			err := pool.PerformStrategically(func(recursor string) error {
				defer func() { completionOrder <- recursor }()

				switch recursor {
				case "fast":
					// Fast recursor - returns immediately
					return nil
				case "slow":
					time.Sleep(50 * time.Millisecond)
					return nil
				case "slower":
					time.Sleep(100 * time.Millisecond)
					return nil
				}
				return errors.New("unknown")
			})

			Expect(err).NotTo(HaveOccurred())
			// Should return fast since it's the first success
			firstCompleted := <-completionOrder
			Expect(firstCompleted).To(Equal("fast"))
		})

		It("waits for all responses when no NOERROR is received and picks best", func() {
			pool := NewFailoverRecursorPool([]string{"servfail", "nxdomain", "neterr"}, config.RaceRecursorSelection, 0, fakeLogger)

			netErr := &net.DNSError{
				Err:       "connection refused",
				IsTimeout: false,
			}

			err := pool.PerformStrategically(func(recursor string) error {
				switch recursor {
				case "servfail":
					// SERVFAIL (priority 2)
					return server.NewDnsError(2, "test.com", recursor) // dns.RcodeServerFailure = 2
				case "nxdomain":
					// NXDOMAIN (priority 1) - should be picked as best
					return server.NewDnsError(3, "test.com", recursor) // dns.RcodeNameError = 3
				case "neterr":
					// Network error (priority 3)
					return netErr
				}
				return nil
			})

			// Should return NXDOMAIN error as it has the best priority (1)
			Expect(err).To(HaveOccurred())
			dnsErr, ok := err.(server.DnsError)
			Expect(ok).To(BeTrue())
			Expect(dnsErr.Rcode()).To(Equal(3)) // dns.RcodeNameError
		})

		It("handles all recursors failing", func() {
			pool := NewFailoverRecursorPool([]string{"one", "two", "three"}, config.RaceRecursorSelection, 0, fakeLogger)

			netErr := &net.DNSError{
				Err:       "i/o timeout",
				IsTimeout: true,
			}

			err := pool.PerformStrategically(func(recursor string) error {
				return netErr
			})

			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(netErr))
		})

		It("handles mixed responses and returns NOERROR when present", func() {
			pool := NewFailoverRecursorPool([]string{"nxdomain", "noerror", "servfail"}, config.RaceRecursorSelection, 0, fakeLogger)

			err := pool.PerformStrategically(func(recursor string) error {
				switch recursor {
				case "nxdomain":
					time.Sleep(20 * time.Millisecond)
					return server.NewDnsError(3, "test.com", recursor) // dns.RcodeNameError
				case "noerror":
					time.Sleep(30 * time.Millisecond)
					return nil // Success!
				case "servfail":
					time.Sleep(10 * time.Millisecond)
					return server.NewDnsError(2, "test.com", recursor) // dns.RcodeServerFailure
				}
				return errors.New("unknown")
			})

			// Even though servfail completes first, we should wait and return NOERROR
			Expect(err).NotTo(HaveOccurred())
		})

		It("can handle concurrent requests", func() {
			pool := NewFailoverRecursorPool([]string{"one", "two"}, config.RaceRecursorSelection, 0, fakeLogger)

			smash := func(done chan struct{}) {
				defer func() { done <- struct{}{} }()
				for i := 0; i < 20; i++ {
					err := pool.PerformStrategically(func(n string) error {
						time.Sleep(time.Millisecond)
						return nil
					})
					Expect(err).NotTo(HaveOccurred())
				}
			}

			done := make(chan struct{})

			go smash(done)
			go smash(done)
			go smash(done)

			for i := 0; i < 3; i++ {
				select {
				case <-done:
					continue
				case <-time.After(time.Minute):
					Fail("reached something like a deadlock")
				}
			}
		})

		It("works with retry logic", func() {
			pool := NewFailoverRecursorPool([]string{"retry1", "retry2"}, config.RaceRecursorSelection, 2, fakeLogger)

			callCount := make(map[string]int)
			var mu sync.Mutex
			netErr := &net.DNSError{
				Err:       "i/o timeout",
				IsTimeout: true,
			}

			err := pool.PerformStrategically(func(recursor string) error {
				mu.Lock()
				callCount[recursor]++
				count := callCount[recursor]
				mu.Unlock()
				// retry1 succeeds on 3rd attempt, retry2 always fails
				if recursor == "retry1" && count >= 3 {
					return nil
				}
				return netErr
			})

			Expect(err).NotTo(HaveOccurred())
			mu.Lock()
			defer mu.Unlock()
			// retry1 should have been called 3 times (initial + 2 retries)
			Expect(callCount["retry1"]).To(Equal(3))
			// retry2 should have been called 3 times as well (runs in parallel)
			Expect(callCount["retry2"]).To(Equal(3))
		})
	})
})
