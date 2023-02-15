package healthconfig_test

import (
	"bosh-dns/healthconfig"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ParseJobs", func() {
	var jobsDir string

	BeforeEach(func() {
		var err error
		jobsDir, err = os.MkdirTemp("", "health-config")
		Expect(err).NotTo(HaveOccurred())

		jobADir := filepath.Join(jobsDir, "job-a")
		err = os.MkdirAll(filepath.Join(jobADir, ".bosh"), 0777)
		Expect(err).NotTo(HaveOccurred())

		f, err := os.Create(filepath.Join(jobADir, ".bosh", "links.json"))
		Expect(err).NotTo(HaveOccurred())

		_, err = f.Write([]byte(`[{"name":"service","type":"connection","group":"1"}]`))
		Expect(err).NotTo(HaveOccurred())

		Expect(f.Close()).To(Succeed())

		jobBDir := filepath.Join(jobsDir, "job-b")
		err = os.MkdirAll(filepath.Join(jobBDir, ".bosh"), 0777)
		Expect(err).NotTo(HaveOccurred())

		f, err = os.Create(filepath.Join(jobBDir, ".bosh", "links.json"))
		Expect(err).NotTo(HaveOccurred())

		_, err = f.Write([]byte(`[{"name":"strudel","type":"dessert","group":"2"}]`))
		Expect(err).NotTo(HaveOccurred())

		Expect(f.Close()).To(Succeed())

		err = os.MkdirAll(filepath.Join(jobBDir, "bin", "dns"), 0777)
		Expect(err).NotTo(HaveOccurred())

		f, err = os.Create(filepath.Join(jobBDir, "bin", "dns", "healthy"))
		Expect(err).NotTo(HaveOccurred())

		Expect(f.Close()).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(jobsDir)).To(Succeed())
	})

	It("parses the jobs directory", func() {
		jobs, err := healthconfig.ParseJobs(jobsDir, "bin/dns/healthy")
		Expect(err).NotTo(HaveOccurred())

		Expect(jobs).To(HaveLen(2))
		Expect(jobs).To(ContainElement(healthconfig.Job{
			HealthExecutablePath: "",
			Groups: []healthconfig.LinkMetadata{{
				Group:   "1",
				Name:    "service",
				Type:    "connection",
				JobName: "job-a",
			}},
		}))

		Expect(jobs).To(ContainElement(healthconfig.Job{
			HealthExecutablePath: filepath.Join(jobsDir, "job-b", "bin", "dns", "healthy"),
			Groups: []healthconfig.LinkMetadata{{
				Group:   "2",
				Name:    "strudel",
				Type:    "dessert",
				JobName: "job-b",
			}},
		}))
	})

	Context("when .bosh directory does not exist", func() {
		BeforeEach(func() {
			jobCDir := filepath.Join(jobsDir, "job-c")
			err := os.MkdirAll(filepath.Join(jobCDir, ".bosh"), 0777)
			Expect(err).NotTo(HaveOccurred())

			err = os.MkdirAll(filepath.Join(jobCDir, "bin", "dns"), 0777)
			Expect(err).NotTo(HaveOccurred())

			f, err := os.Create(filepath.Join(jobCDir, "bin", "dns", "healthy"))
			Expect(err).NotTo(HaveOccurred())
			Expect(f.Close()).To(Succeed())
		})

		It("parses the job successfully", func() {
			jobs, err := healthconfig.ParseJobs(jobsDir, "bin/dns/healthy")
			Expect(err).NotTo(HaveOccurred())

			Expect(jobs).To(HaveLen(3))
			Expect(jobs).To(ContainElement(healthconfig.Job{
				HealthExecutablePath: filepath.Join(jobsDir, "job-c", "bin", "dns", "healthy"),
				Groups:               []healthconfig.LinkMetadata{},
			}))
		})
	})

	Context("when the link metatdata has invalid json", func() {
		BeforeEach(func() {
			jobCDir := filepath.Join(jobsDir, "job-c")

			err := os.MkdirAll(filepath.Join(jobCDir, ".bosh"), 0777)
			Expect(err).NotTo(HaveOccurred())

			f, err := os.Create(filepath.Join(jobCDir, ".bosh", "links.json"))
			Expect(err).NotTo(HaveOccurred())

			_, err = f.Write([]byte(`{{`))
			Expect(err).NotTo(HaveOccurred())
			Expect(f.Close()).To(Succeed())
		})

		It("returns an error", func() {
			_, err := healthconfig.ParseJobs(jobsDir, "bin/dns/healthy")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when the job directory does not exist", func() {
		It("returns an error", func() {
			_, err := healthconfig.ParseJobs("bogus-director-yo", "bin/dns/healthy")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when the jobs directory contains a file", func() {
		BeforeEach(func() {
			f, err := os.Create(filepath.Join(jobsDir, "foobar"))
			Expect(err).NotTo(HaveOccurred())
			Expect(f.Close()).To(Succeed())
		})

		It("ignores the file", func() {
			jobs, err := healthconfig.ParseJobs(jobsDir, "bin/dns/healthy")
			Expect(err).NotTo(HaveOccurred())

			Expect(jobs).To(HaveLen(2))
			Expect(jobs).To(ContainElement(healthconfig.Job{
				HealthExecutablePath: "",
				Groups: []healthconfig.LinkMetadata{{
					Group:   "1",
					Name:    "service",
					Type:    "connection",
					JobName: "job-a",
				}},
			}))

			Expect(jobs).To(ContainElement(healthconfig.Job{
				HealthExecutablePath: filepath.Join(jobsDir, "job-b", "bin", "dns", "healthy"),
				Groups: []healthconfig.LinkMetadata{{
					Group:   "2",
					Name:    "strudel",
					Type:    "dessert",
					JobName: "job-b",
				}},
			}))
		})
	})
})
