package records_test

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	"github.com/cloudfoundry/dns-release/src/dns/server/records"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("Repo", func() {

	Describe("NewRepo", func() {
		var (
			repo        *records.Repo
			fakeLogger  = &loggerfakes.FakeLogger{}
		)

		Context("initial failure cases", func() {
			It("logs an error when the file does not exist", func() {
				repo = records.NewRepo("file-does-not-exist", fakeLogger)
				Expect(fakeLogger.ErrorCallCount()).To(Equal(1))

				tag, message, _ := fakeLogger.ErrorArgsForCall(0)
				Expect(tag).To(Equal("RecordsRepo"))
				Expect(message).To(Equal("Unable to open records file at: file-does-not-exist"))
			})
		})
	})

	Describe("Get", func() {
		var (
			recordsFile *os.File
			repo        *records.Repo
			fakeLogger  = &loggerfakes.FakeLogger{}
		)

		BeforeEach(func() {
			var err error
			recordsFile, err = ioutil.TempFile("", "")
			Expect(err).NotTo(HaveOccurred())

			_, err = recordsFile.Write([]byte(fmt.Sprint(`{
				"record_keys": ["id", "instance_group", "az", "network", "deployment", "ip"],
				"record_infos": [
					["my-instance", "my-group", "az1", "my-network", "my-deployment", "123.123.123.123"],
					["my-instance", "my-group", "az1", "my-network", "my-deployment", "123.123.123.124"]
				]
			}`)))
			Expect(err).NotTo(HaveOccurred())

			repo = records.NewRepo(recordsFile.Name(), fakeLogger)
		})

		Context("initial failure cases", func() {
			It("returns an error when the file does not exist", func() {
				repo := records.NewRepo("/some/fake/path", fakeLogger)
				_, err := repo.Get()
				Expect(err).To(MatchError("open /some/fake/path: no such file or directory"))
			})

			It("returns an error when the file is malformed json", func() {
				recordsFile, err := ioutil.TempFile("", "")
				Expect(err).NotTo(HaveOccurred())

				_, err = recordsFile.Write([]byte(fmt.Sprint(`invalid json`)))
				Expect(err).NotTo(HaveOccurred())

				repo := records.NewRepo(recordsFile.Name(), fakeLogger)
				_, err = repo.Get()
				Expect(err).To(MatchError("invalid character 'i' looking for beginning of value"))
			})
		})

		Context("when there are records matching the specified fqdn", func() {
			It("returns all records for that name", func() {
				recordSet, err := repo.Get()
				Expect(err).NotTo(HaveOccurred())

				records, err := recordSet.Resolve("my-instance.my-group.my-network.my-deployment.bosh.")
				Expect(err).NotTo(HaveOccurred())

				Expect(records).To(ContainElement("123.123.123.123"))
				Expect(records).To(ContainElement("123.123.123.124"))
			})
		})

		Context("when there are no records matching the specified fqdn", func() {
			It("returns an empty set of records", func() {
				recordSet, err := repo.Get()
				Expect(err).NotTo(HaveOccurred())
				records, err := recordSet.Resolve("some.garbage.fqdn.deploy.bosh")
				Expect(err).NotTo(HaveOccurred())

				Expect(records).To(HaveLen(0))
			})
		})

		Context("when viability of file has changed", func() {
			Context("when records json file has been re-added with different contents after getting initial values", func() {
				BeforeEach(func() {
					_, err := repo.Get()
					Expect(err).NotTo(HaveOccurred())

					err = ioutil.WriteFile(recordsFile.Name(), []byte(`{
				"record_keys": ["id", "instance_group", "az", "network", "deployment", "ip"],
				"record_infos": [
					["my-instance2", "my-group", "az1", "my-network", "my-deployment", "123.123.123.123"],
					["my-instance2", "my-group", "az1", "my-network", "my-deployment", "123.123.123.124"]
				]
			}`), os.ModePerm)
					Expect(err).NotTo(HaveOccurred())

					err = os.Chtimes(recordsFile.Name(), time.Time{}, time.Time{})
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return all records from new file contents", func() {
					recordSet, err := repo.Get()
					Expect(err).NotTo(HaveOccurred())

					records, err := recordSet.Resolve("my-instance.my-group.my-network.my-deployment.bosh.")
					Expect(err).NotTo(HaveOccurred())

					Expect(records).To(ContainElement("123.123.123.123"))
					Expect(records).To(ContainElement("123.123.123.124"))
				})
			})

			Context("when file has been deleted after repo initialization", func() {
				BeforeEach(func() {
					os.Remove(recordsFile.Name())
				})

				It("should return all records from original file contents", func() {
					recordSet, err := repo.Get()
					Expect(err).NotTo(HaveOccurred())

					records, err := recordSet.Resolve("my-instance.my-group.my-network.my-deployment.bosh.")
					Expect(err).NotTo(HaveOccurred())

					Expect(records).To(ContainElement("123.123.123.123"))
					Expect(records).To(ContainElement("123.123.123.124"))
				})

				Context("when records json file has been re-added with different contents", func() {
					BeforeEach(func() {
						err := ioutil.WriteFile(recordsFile.Name(), []byte(`{
				"record_keys": ["id", "instance_group", "az", "network", "deployment", "ip"],
				"record_infos": [
					["my-instance2", "my-group", "az1", "my-network", "my-deployment", "123.123.123.123"],
					["my-instance2", "my-group", "az1", "my-network", "my-deployment", "123.123.123.124"]
				]
			}`), os.ModePerm)

						Expect(err).NotTo(HaveOccurred())
					})

					It("should return all records from new file contents", func() {
						recordSet, err := repo.Get()
						Expect(err).NotTo(HaveOccurred())

						records, err := recordSet.Resolve("my-instance2.my-group.my-network.my-deployment.bosh.")
						Expect(err).NotTo(HaveOccurred())

						Expect(records).To(ContainElement("123.123.123.123"))
						Expect(records).To(ContainElement("123.123.123.124"))
					})
				})
			})
		})
	})
})
