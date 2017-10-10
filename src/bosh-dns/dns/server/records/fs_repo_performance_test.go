package records_test

import (
	"io/ioutil"
	"sort"
	"sync"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"

	. "bosh-dns/dns/server/records"

	blogfakes "github.com/cloudfoundry/bosh-utils/logger/fakes"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	"code.cloudfoundry.org/clock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("FsRepoPerformance", func() {
	var (
		start, done     chan struct{}
		recordsFilePath string
		repo            RecordSetProvider
		fileSys         boshsys.FileSystem
		clock           clock.Clock
		logger          *blogfakes.FakeLogger
		mutex           *sync.Mutex
	)

	BeforeEach(func() {
		mutex = &sync.Mutex{}

		start = make(chan struct{})
		done = make(chan struct{})

		clock = fakeclock.NewFakeClock(time.Now())

		recordsFile, err := ioutil.TempFile("", "records")
		Expect(err).NotTo(HaveOccurred())
		recordsFilePath = recordsFile.Name()

		recordsFile.Write([]byte(`{"zones":["my.domain.","your.domain.","example.com."]}`))

		logger = &blogfakes.FakeLogger{}
		fileSys = boshsys.NewOsFileSystem(logger)
	})

	Context("using mutex locks", func() {
		BeforeEach(func() {
			repo = NewRepo(recordsFilePath, fileSys, clock, logger, done)
		})

		It("should have a median response time less than 0.01 ms with no writes", func() {
			values := []float64{}
			hammerGet(repo, start, done, mutex, func(f float64) {
				values = append(values, f)
			})

			close(start)

			time.Sleep(2 * time.Second)

			close(done)

			mutex.Lock()
			sort.Float64s(values)
			median := values[len(values)/2]
			mutex.Unlock()

			Expect(median).To(BeNumerically("<", 0.01))
		})

		It("should have a median response time less than 0.01 ms with periodic writes", func() {
			values := []float64{}
			hammerGet(repo, start, done, mutex, func(f float64) {
				values = append(values, f)
			})

			go func() {
				defer GinkgoRecover()
				close(start)

				time.Sleep(2 * time.Second)

				close(done)
			}()

		dance:
			for {
				select {
				case <-done:
					break dance
				default:
					Expect(fileSys.WriteFileString(recordsFilePath, `{"zones":["my.domain.","your.domain.","example.com."]}`)).To(Succeed())
				}
			}

			mutex.Lock()
			sort.Float64s(values)
			median := values[len(values)/2]
			mutex.Unlock()

			Expect(median).To(BeNumerically("<", 0.01))
		})
	})
})

func hammerGet(repo RecordSetProvider, start, done chan struct{}, mutex *sync.Mutex, benchmark func(float64)) {
	for i := 0; i < 1000; i++ {
		go func(s chan struct{}, r RecordSetProvider, d chan struct{}) {
			defer GinkgoRecover()
			<-s
			for {
				select {
				case <-d:
					return
				default:
					before := time.Now()
					_, err := r.Get()
					after := time.Now()

					Expect(err).NotTo(HaveOccurred())
					mutex.Lock()
					benchmark(float64(after.Sub(before)) / float64(time.Millisecond))
					mutex.Unlock()
				}
			}
		}(start, repo, done)
	}
}
