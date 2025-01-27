package records_test

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"bosh-dns/dns/server/aliases"
	"bosh-dns/dns/server/healthiness/healthinessfakes"
	"bosh-dns/dns/server/records"
	"bosh-dns/dns/server/records/recordsfakes"
	"bosh-dns/healthcheck/api"
)

var _ = Describe("Record Set Performance", func() {
	const expectedAvergeResolutionTime int64 = 5000 // 5 milliseconds

	var (
		recordSet             *records.RecordSet
		fakeLogger            *loggerfakes.FakeLogger
		fileReader            *recordsfakes.FakeFileReader
		aliasList             aliases.Config
		shutdownChan          chan struct{}
		fakeHealthWatcher     *healthinessfakes.FakeHealthWatcher
		filtererFactory       records.FiltererFactory
		fakeAliasQueryEncoder *recordsfakes.FakeAliasQueryEncoder
	)

	BeforeEach(func() {
		fakeLogger = &loggerfakes.FakeLogger{}
		fileReader = &recordsfakes.FakeFileReader{}
		fakeAliasQueryEncoder = &recordsfakes.FakeAliasQueryEncoder{}

		aliasList = mustNewConfigFromMap(map[string][]string{})
		fakeHealthWatcher = &healthinessfakes.FakeHealthWatcher{}
		filtererFactory = records.NewHealthFiltererFactory(fakeHealthWatcher, time.Second)
		shutdownChan = make(chan struct{})
		fakeHealthWatcher.HealthStateReturns(api.HealthResult{State: api.StatusRunning})
	})

	AfterEach(func() {
		close(shutdownChan)
	})
	BeforeEach(func() {
		recordData := [][]string{[]string{
			"instance0",
			"0",
			"my-group",
			"az4",
			"4",
			"my-network",
			"1",
			"my-deployment",
			"123.123.1.0",
			"domain.",
		}}

		for i := 1; i < 2000; i++ {
			recordData = append(recordData, []string{
				fmt.Sprintf("instance%d", i),
				fmt.Sprintf("%d", i),
				"my-group",
				fmt.Sprintf("az%d", i%3),
				fmt.Sprintf("%d", i%3),
				"my-network",
				"1",
				"my-deployment",
				fmt.Sprintf("123.123.%d.%d", (i+1)%256, i%256),
				"domain.",
			})
		}

		recordInfosJson, err := json.Marshal(recordData)
		Expect(err).NotTo(HaveOccurred())

		jsonBytes := []byte(fmt.Sprintf(`{
			"record_keys": ["id", "num_id", "instance_group", "az", "az_id", "network", "network_id", "deployment", "ip", "domain"],
				"record_infos": %s
			}`, recordInfosJson))

		fileReader.GetReturns(jsonBytes, nil)

		recordSet, err = records.NewRecordSet(fileReader, aliasList, fakeHealthWatcher, uint(5), shutdownChan, fakeLogger, filtererFactory, fakeAliasQueryEncoder)

		Expect(err).ToNot(HaveOccurred())
	})

	It(fmt.Sprintf("resolves queries with a large number of records under %d nanoseconds", expectedAvergeResolutionTime), func() {
		var totalTime time.Duration
		var totalTimeLastRecord time.Duration
		var count int

		for count = 0; count < 1000; count++ {
			startTime := time.Now()
			ips, err := recordSet.Resolve("q-m0s0.my-group.my-network.my-deployment.domain.")
			totalTime += time.Since(startTime)

			Expect(err).ToNot(HaveOccurred())
			Expect(ips).To(HaveExactElements("123.123.1.0"))

			startTime = time.Now()
			ips, err = recordSet.Resolve("q-m1999s0.my-group.my-network.my-deployment.domain.")
			totalTimeLastRecord += time.Since(startTime)

			Expect(err).ToNot(HaveOccurred())
			Expect(ips).To(HaveExactElements("123.123.208.207"))
		}

		averageTime := totalTime.Microseconds() / int64(count)
		averageTimeLastRecord := totalTimeLastRecord.Microseconds() / int64(count)

		Expect(averageTime).To(BeNumerically("<", expectedAvergeResolutionTime))
		Expect(averageTimeLastRecord).To(BeNumerically("<", expectedAvergeResolutionTime))
	})
})
