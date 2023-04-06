package records_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	"github.com/cloudfoundry/bosh-utils/system/fakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"bosh-dns/dns/server/records"
)

var _ = Describe("RecordsFileReader", func() {
	var (
		shutdownChan   chan struct{}
		recordsFile    boshsys.File
		fileReader     records.FileReader
		fakeClock      *fakeclock.FakeClock
		fakeLogger     *loggerfakes.FakeLogger
		fakeFileSystem *fakes.FakeFileSystem
		fileContents   string
	)

	BeforeEach(func() {
		shutdownChan = make(chan struct{})
		fakeFileSystem = fakes.NewFakeFileSystem()
		fakeClock = fakeclock.NewFakeClock(time.Now())
		fakeLogger = &loggerfakes.FakeLogger{}
		recordsFile = fakes.NewFakeFile("/fake/file", fakeFileSystem)

		fileContents = `{
				"record_keys": ["id", "instance_group", "az", "network", "deployment", "ip", "domain"],
				"record_infos": [
					["my-instance", "my-group", "az1", "my-network", "my-deployment", "123.123.123.123", "my-domain"],
					["my-instance", "my-group", "az1", "my-network", "my-deployment", "123.123.123.124", "my-domain"]
				]
			}`
		err := fakeFileSystem.WriteFile(recordsFile.Name(), []byte(fileContents))
		Expect(err).NotTo(HaveOccurred())

		fileReader = records.NewFileReader(recordsFile.Name(), fakeFileSystem, fakeClock, fakeLogger, shutdownChan)
	})

	AfterEach(func() {
		close(shutdownChan)
	})

	Describe("NewRepo", func() {
		var (
			nonExistentFilePath string
		)

		BeforeEach(func() {
			nonExistentFilePath = "/some/fake/path"
			fakeFileSystem.RegisterOpenFile(nonExistentFilePath, &fakes.FakeFile{
				StatErr: errors.New("NOPE"),
			})
		})

		Context("initial failure cases", func() {
			It("logs an error when the file does not exist", func() {
				fileReader = records.NewFileReader("/some/fake/path", fakeFileSystem, fakeClock, fakeLogger, shutdownChan)
				Expect(fakeLogger.ErrorCallCount()).To(Equal(1))

				tag, message, _ := fakeLogger.ErrorArgsForCall(0)
				Expect(tag).To(Equal("RecordsRepo"))
				Expect(message).To(Equal("Unable to open records file at: /some/fake/path"))
			})
		})
	})

	Describe("Get", func() {
		Context("initial failure cases", func() {
			It("returns an error when the file does not exist", func() {
				nonExistentFilePath := "/some/fake/path"
				fakeFileSystem.RegisterOpenFile(nonExistentFilePath, &fakes.FakeFile{
					StatErr: errors.New("NOPE"),
				})

				repo := records.NewFileReader(nonExistentFilePath, fakeFileSystem, fakeClock, fakeLogger, shutdownChan)
				_, err := repo.Get()
				Expect(err).To(MatchError("Error stating records file '/some/fake/path': NOPE"))
			})

			It("returns an error when a file read error occurs", func() {
				fakeFileSystem.RegisterReadFileError(recordsFile.Name(), errors.New("can not read file"))

				repo := records.NewFileReader(recordsFile.Name(), fakeFileSystem, fakeClock, fakeLogger, shutdownChan)
				_, err := repo.Get()
				Expect(err).To(MatchError("can not read file"))
			})
		})

		It("returns the contents of the file", func() {
			contents, err := fileReader.Get()
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeFileSystem.StatWithOptsCallCount).To(Equal(1))

			Expect(string(contents)).To(Equal(fileContents))
		})

		Context("when viability of file has changed", func() {
			Context("when records json file has been re-added with different contents after getting initial values", func() {
				var newFileContents string

				BeforeEach(func() {
					_, err := fileReader.Get()
					Expect(err).NotTo(HaveOccurred())

					newFileContents = `{
						"record_keys": ["id", "instance_group", "az", "network", "deployment", "ip", "domain"],
						"record_infos": [
							["my-instance2", "my-group", "az1", "my-network", "my-deployment", "123.123.123.128", "my-domain"],
							["my-instance2", "my-group", "az1", "my-network", "my-deployment", "123.123.123.129", "my-domain"]
						]
					}`
					err = fakeFileSystem.WriteFile(recordsFile.Name(), []byte(newFileContents))
					Expect(err).NotTo(HaveOccurred())

					fakeFileSystem.RegisterOpenFile(recordsFile.Name(), &fakes.FakeFile{
						Stats: &fakes.FakeFileStats{
							ModTime: fakeClock.Now(),
						},
					})

					// Waiting the first time will trigger the clock.Sleep in the
					// cache auto-update thread. Waiting the second time will
					// ensure that the auto-update thread has finished its previous
					// iteration.
					fakeClock.WaitForWatcherAndIncrement(time.Second)
					fakeClock.WaitForWatcherAndIncrement(0)
				})

				It("returns the new contents", func() {
					contents, err := fileReader.Get()
					Expect(err).NotTo(HaveOccurred())

					Expect(string(contents)).To(Equal(newFileContents))
				})
			})

			// Context("when the file changes", func() {
			// 	var initialTime time.Time
			// 	BeforeEach(func() {
			// 		initialTime = time.Now()

			// 		err := fakeFileSystem.WriteFile(recordsFile.Name(), []byte(`{
			// 			"record_keys": ["id", "instance_group", "az", "network", "deployment", "ip", "domain"],
			// 			"record_infos": [
			// 				["my-instance2", "my-group", "az1", "my-network", "my-deployment", "123.123.123.123", "my-domain"],
			// 				["my-instance2", "my-group", "az1", "my-network", "my-deployment", "123.123.123.124", "my-domain"]
			// 			]
			// 		}`))
			// 		Expect(err).NotTo(HaveOccurred())

			// 		fakeFileSystem.RegisterOpenFile(recordsFile.Name(), &fakes.FakeFile{
			// 			Stats: &fakes.FakeFileStats{
			// 				ModTime: initialTime.Add(-3 * time.Second),
			// 			},
			// 		})

			// 		fakeClock.WaitForWatcherAndIncrement(time.Second)
			// 		fakeClock.WaitForWatcherAndIncrement(0)

			// 		_, err = repo.Get()
			// 		Expect(err).NotTo(HaveOccurred())
			// 	})

			// Context("new file is malformed/incomplete", func() {
			// 	BeforeEach(func() {
			// 		err := fakeFileSystem.WriteFile(recordsFile.Name(), []byte(`{
			// 			"record_keys": ["id", "instance_group", "az", "network", "deployment", "domain", "ip"],
			// 			"record_in`))
			// 		Expect(err).NotTo(HaveOccurred())

			// 		fakeFileSystem.RegisterOpenFile(recordsFile.Name(), &fakes.FakeFile{
			// 			Stats: &fakes.FakeFileStats{
			// 				ModTime: initialTime.Add(-1 * time.Second),
			// 			},
			// 		})

			// 		fakeClock.WaitForWatcherAndIncrement(time.Second)
			// 		fakeClock.WaitForWatcherAndIncrement(time.Second)
			// 	})

			// 	It("returns the cached content", func() {
			// 		recordSet, err := repo.Get()
			// 		Expect(err).NotTo(HaveOccurred())

			// 		records, err := recordSet.Resolve("my-instance2.my-group.my-network.my-deployment.my-domain.")
			// 		Expect(err).NotTo(HaveOccurred())

			// 		Expect(records).To(ContainElement("123.123.123.123"))
			// 		Expect(records).To(ContainElement("123.123.123.124"))
			// 	})
			// })

			Context("when the file becomes unreadable", func() {
				var initialTime time.Time

				BeforeEach(func() {
					initialTime = time.Now()

					fakeFileSystem.RegisterOpenFile(recordsFile.Name(), &fakes.FakeFile{
						Stats: &fakes.FakeFileStats{
							ModTime: initialTime.Add(-3 * time.Second),
						},
					})

					fakeClock.WaitForWatcherAndIncrement(time.Second)
					fakeClock.WaitForWatcherAndIncrement(0)

					_, err := fileReader.Get()
					Expect(err).NotTo(HaveOccurred())

					fakeFileSystem.RegisterOpenFile(recordsFile.Name(), &fakes.FakeFile{
						Stats: &fakes.FakeFileStats{
							ModTime: initialTime.Add(-2 * time.Second),
						},
					})

					fakeFileSystem.RegisterReadFileError(recordsFile.Name(), errors.New("some read file error"))
				})

				It("should return the cached content", func() {
					contents, err := fileReader.Get()
					Expect(err).NotTo(HaveOccurred())

					Expect(string(contents)).To(Equal(fileContents))
				})

				Context("when the file becomes readable again", func() {
					var newFileContents string
					BeforeEach(func() {
						fakeFileSystem.UnregisterReadFileError(recordsFile.Name())

						newFileContents = `{
							"record_keys": ["id", "instance_group", "az", "network", "deployment", "ip", "domain"],
							"record_infos": [
								["my-instance2", "my-group", "az1", "my-network", "my-deployment", "1.2.3.4", "my-domain"],
								["my-instance2", "my-group", "az1", "my-network", "my-deployment", "1.2.3.5", "my-domain"]
							]
						}`
						err := fakeFileSystem.WriteFile(recordsFile.Name(), []byte(newFileContents))

						Expect(err).NotTo(HaveOccurred())

						fakeClock.WaitForWatcherAndIncrement(time.Second)
						fakeClock.WaitForWatcherAndIncrement(0)

						fakeFileSystem.RegisterOpenFile(recordsFile.Name(), &fakes.FakeFile{
							Stats: &fakes.FakeFileStats{
								ModTime: initialTime.Add(-1 * time.Second),
							},
						})
					})

					It("returns contents from new file contents", func() {
						contents, err := fileReader.Get()
						Expect(err).NotTo(HaveOccurred())

						Expect(string(contents)).To(Equal(newFileContents))
					})
				})
			})

			Context("when the file becomes un stat able", func() {
				var initialTime time.Time

				BeforeEach(func() {
					initialTime = time.Now()

					fakeFileSystem.RegisterOpenFile(recordsFile.Name(), &fakes.FakeFile{
						Stats: &fakes.FakeFileStats{
							ModTime: initialTime.Add(-3 * time.Second),
						},
					})

					fakeClock.WaitForWatcherAndIncrement(time.Second)
					fakeClock.WaitForWatcherAndIncrement(0)

					_, err := fileReader.Get()
					Expect(err).NotTo(HaveOccurred())

					fakeFileSystem.RegisterOpenFile(recordsFile.Name(), &fakes.FakeFile{
						StatErr: errors.New("stat err"),
					})
				})

				It("should return the cached content", func() {
					contents, err := fileReader.Get()
					Expect(err).NotTo(HaveOccurred())

					Expect(string(contents)).To(Equal(fileContents))
				})

				Context("when the file becomes stat able again", func() {
					var newFileContents string

					BeforeEach(func() {
						newFileContents = `{
							"record_keys": ["id", "instance_group", "az", "network", "deployment", "ip", "domain"],
							"record_infos": [
								["my-instance2", "my-group", "az1", "my-network", "my-deployment", "1.2.3.4", "my-domain"],
								["my-instance2", "my-group", "az1", "my-network", "my-deployment", "1.2.3.5", "my-domain"]
							]
						}`
						err := fakeFileSystem.WriteFile(recordsFile.Name(), []byte(newFileContents))

						Expect(err).NotTo(HaveOccurred())

						fakeFileSystem.RegisterOpenFile(recordsFile.Name(), &fakes.FakeFile{
							Stats: &fakes.FakeFileStats{
								ModTime: initialTime.Add(-1 * time.Second),
							},
						})

						fakeClock.WaitForWatcherAndIncrement(time.Second)
						fakeClock.WaitForWatcherAndIncrement(0)
					})

					It("returns contents from new file contents", func() {
						contents, err := fileReader.Get()
						Expect(err).NotTo(HaveOccurred())

						Expect(string(contents)).To(Equal(newFileContents))
					})
				})
			})

			Context("when file has been deleted after repo initialization", func() {
				BeforeEach(func() {
					err := fakeFileSystem.RemoveAll(recordsFile.Name())
					Expect(err).ToNot(HaveOccurred())

					fakeFileSystem.RegisterOpenFile(recordsFile.Name(), &fakes.FakeFile{
						StatErr: errors.New("file does not exist"),
					})
				})

				It("returns contents from original file contents", func() {
					contents, err := fileReader.Get()
					Expect(err).NotTo(HaveOccurred())

					Expect(string(contents)).To(Equal(fileContents))
				})

				Context("when file has been re-added with different contents", func() {
					var newFileContents string

					BeforeEach(func() {
						newFileContents = `{
							"record_keys": ["id", "instance_group", "az", "network", "deployment", "ip", "domain"],
							"record_infos": [
								["my-instance2", "my-group", "az1", "my-network", "my-deployment", "123.123.123.123", "my-domain"],
								["my-instance2", "my-group", "az1", "my-network", "my-deployment", "123.123.123.124", "my-domain"]
							]
						}`
						err := fakeFileSystem.WriteFile(recordsFile.Name(), []byte(newFileContents))

						Expect(err).NotTo(HaveOccurred())

						fakeFileSystem.RegisterOpenFile(recordsFile.Name(), &fakes.FakeFile{
							Stats: &fakes.FakeFileStats{
								ModTime: time.Now(),
							},
						})

						fakeClock.WaitForWatcherAndIncrement(time.Second)
						fakeClock.WaitForWatcherAndIncrement(0)
					})

					It("returns contents from new file contents", func() {
						contents, err := fileReader.Get()
						Expect(err).NotTo(HaveOccurred())

						Expect(string(contents)).To(Equal(newFileContents))
					})
				})
			})
		})
	})

	Describe("Subscribe", func() {
		It("notifies when changes occur", func() {
			c := fileReader.Subscribe()

			err := fakeFileSystem.WriteFile(recordsFile.Name(), []byte(`{
				"record_keys": ["id", "instance_group", "az", "network", "deployment", "ip", "domain"],
				"record_infos": [
					["my-instance2", "my-group", "az1", "my-network", "my-deployment", "123.123.123.128", "my-domain"],
					["my-instance2", "my-group", "az1", "my-network", "my-deployment", "123.123.123.129", "my-domain"]
				]
			}`))
			Expect(err).NotTo(HaveOccurred())

			fakeFileSystem.RegisterOpenFile(recordsFile.Name(), &fakes.FakeFile{
				Stats: &fakes.FakeFileStats{
					ModTime: fakeClock.Now(),
				},
			})

			Consistently(c).ShouldNot(Receive())
			fakeClock.WaitForWatcherAndIncrement(time.Second)
			Eventually(c).Should(Receive())
		})
	})
})
