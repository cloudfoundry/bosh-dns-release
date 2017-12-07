package internal_test

import (
	. "bosh-dns/dns/server/healthiness/internal"
	"sync"

	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PriorityLimitedTranscript", func() {
	var (
		transcript *PriorityLimitedTranscript
	)

	BeforeEach(func() {
		transcript = NewPriorityLimitedTranscript(5)
	})

	It("maintains registered names", func() {
		Expect(transcript.Touch("one")).To(Equal(""))
		Expect(transcript.Touch("two")).To(Equal(""))
		Expect(transcript.Touch("three")).To(Equal(""))

		Expect(transcript.Registry()).To(ConsistOf([]string{"one", "two", "three"}))
	})

	It("throws out the oldest registrations after size limit reached", func() {
		Expect(transcript.Touch("one")).To(Equal(""))
		Expect(transcript.Touch("two")).To(Equal(""))
		Expect(transcript.Touch("three")).To(Equal(""))
		Expect(transcript.Touch("four")).To(Equal(""))
		Expect(transcript.Touch("five")).To(Equal(""))

		Expect(transcript.Touch("six")).To(Equal("one"))
		Expect(transcript.Touch("seven")).To(Equal("two"))
		Expect(transcript.Touch("eight")).To(Equal("three"))

		Expect(transcript.Registry()).To(ConsistOf([]string{
			"four",
			"five",
			"six",
			"seven",
			"eight",
		}))
	})

	It("will throw out least recently used rather than least recently added", func() {
		Expect(transcript.Touch("one")).To(Equal(""))
		Expect(transcript.Touch("two")).To(Equal(""))
		Expect(transcript.Touch("three")).To(Equal(""))
		Expect(transcript.Touch("four")).To(Equal(""))
		Expect(transcript.Touch("five")).To(Equal(""))
		Expect(transcript.Touch("one")).To(Equal(""))

		Expect(transcript.Touch("six")).To(Equal("two"))
		Expect(transcript.Touch("seven")).To(Equal("three"))
		Expect(transcript.Touch("eight")).To(Equal("four"))

		Expect(transcript.Registry()).To(ConsistOf([]string{
			"one",
			"five",
			"six",
			"seven",
			"eight",
		}))
	})

	It("is threadsafe", func() {
		done := make(chan struct{})
		wg := sync.WaitGroup{}

		touch := func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					Expect(transcript.Touch("one")).To(Equal(""))
				}
			}
		}

		wg.Add(2)
		go touch()
		go touch()

		time.Sleep(59 * time.Microsecond)

		close(done)
		wg.Wait()
	})
})
