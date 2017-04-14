package records_test

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/cloudfoundry/dns-release/src/dns/server/records"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Repo", func() {
	Context("GetIPs", func() {
		var (
			recordsFile *os.File
			repo        records.Repo
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

			repo = records.NewRepo(recordsFile.Name())
		})

		Context("failure cases", func() {
			It("returns an error when the file does not exist", func() {
				repo := records.NewRepo("/some/fake/path")
				_, err := repo.Get()
				Expect(err).To(MatchError("open /some/fake/path: no such file or directory"))
			})

			It("returns an error when the file is malformed json", func() {
				recordsFile, err := ioutil.TempFile("", "")
				Expect(err).NotTo(HaveOccurred())

				_, err = recordsFile.Write([]byte(fmt.Sprint(`invalid json`)))
				Expect(err).NotTo(HaveOccurred())

				repo := records.NewRepo(recordsFile.Name())
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
	})
})
