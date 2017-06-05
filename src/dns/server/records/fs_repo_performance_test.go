package records_test

import (
	"io/ioutil"
	"sort"
	"sync"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"

	. "github.com/cloudfoundry/dns-release/src/dns/server/records"

	blogfakes "github.com/cloudfoundry/bosh-utils/logger/fakes"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("FsRepoPerformance", func() {
	var (
		start, done     chan struct{}
		recordsFilePath string
		repo            RecordSetProvider
		fileSys         boshsys.FileSystem
		clock= fakeclock.NewFakeClock(time.Now())
		logger          *blogfakes.FakeLogger
	)

	BeforeEach(func() {
		start = make(chan struct{})
		done = make(chan struct{})

		recordsFile, err := ioutil.TempFile("/tmp", "records")
		Expect(err).NotTo(HaveOccurred())
		recordsFilePath = recordsFile.Name()

		recordsFile.Write([]byte(`{"zones":["my.domain.","your.domain.","example.com."]}`))

		logger = &blogfakes.FakeLogger{}
		fileSys = boshsys.NewOsFileSystem(logger)
	})

	Context("using mutex locks", func() {
		BeforeEach(func() {
			repo = NewRepo(recordsFilePath, fileSys, clock, logger)
		})

		It("should have a median response time less than 0.01 ms with no writes", func() {
			values := []float64{}
			hammerGet(repo, start, done, func(f float64) {
				values = append(values, f)
			})

			close(start)

			time.Sleep(2 * time.Second)

			close(done)
			sort.Float64s(values)
			median := values[len(values)/2]
		  Expect(median).To(BeNumerically("<", 0.001))
		})

		It("should have a median response time less than 0.01 ms with periodic writes", func() {
			values := []float64{}
			hammerGet(repo, start, done, func(f float64) {
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
			sort.Float64s(values)
			median := values[len(values)/2]
			Expect(median).To(BeNumerically("<", 0.001))
		})
	})
})

func hammerGet(repo RecordSetProvider, start, done chan struct{}, benchmark func(float64)) {
	mutex := &sync.Mutex{}
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
