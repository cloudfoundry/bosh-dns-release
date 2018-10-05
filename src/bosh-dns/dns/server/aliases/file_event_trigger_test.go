package aliases_test

import (
	. "bosh-dns/dns/server/aliases"
	"fmt"
	"time"

	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	boshsysfakes "github.com/cloudfoundry/bosh-utils/system/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("FileEventTrigger", func() {
	var (
		fakeFS           *boshsysfakes.FakeFileSystem
		fileEventTrigger *FileEventTrigger
		fakeLogger       *loggerfakes.FakeLogger
		testFile         string
	)

	BeforeEach(func() {
		testFile = "/xiaolongxia.aliases"
		fakeFS = boshsysfakes.NewFakeFileSystem()
		fakeFS.SetGlob("/*.aliases", []string{testFile})
		fakeLogger = &loggerfakes.FakeLogger{}
		fileEventTrigger = NewFileEventTrigger(fakeLogger, fakeFS, "/*.aliases", time.Second)
	})

	Context("Added", func() {
		var (
			subscriber <-chan FileEvent
		)

		It("added notifies file event subscriber", func() {
			subscriber = fileEventTrigger.Subscribe()
			fakeFS.WriteFile(testFile, []byte(fmt.Sprint(`{
				"master.cfcr.internal":["*.master.default.my-cluster.bosh"],
				"master-0.etcd.cfcr.internal":["35dabd21-18f0-4341-a808-0365bab0f6ca.master.default.my-cluster.bosh"]
				}`)))
			go fileEventTrigger.Start()
			var fileEvent FileEvent
			Eventually(subscriber, time.Second*3).Should(Receive(&fileEvent))
			Ω(fileEvent.Type).Should(Equal(Added))
			Ω(fileEvent.File).Should(Equal(testFile))
		})
	})

	Context("Updated", func() {
		var (
			subscriber <-chan FileEvent
		)

		It("updated notifies file event subscriber", func() {
			subscriber = fileEventTrigger.Subscribe()
			fakeFS.WriteFile(testFile, []byte(fmt.Sprint(`{}`)))
			go fileEventTrigger.Start()
			var fileEvent FileEvent

			Eventually(subscriber, time.Second*3).Should(Receive(&fileEvent))
			Ω(fileEvent.Type).Should(Equal(Added))
			Ω(fileEvent.File).Should(Equal(testFile))
			fakeFS.WriteFile(testFile, []byte(fmt.Sprint(`{
				"master.cfcr.internal":["*.master.default.my-cluster.bosh"],
				"master-0.etcd.cfcr.internal":["35dabd21-18f0-4341-a808-0365bab0f6ca.master.default.my-cluster.bosh"]
				}`)))
			Eventually(subscriber, time.Second*3).Should(Receive(&fileEvent))
			Ω(fileEvent.Type).Should(Equal(Updated))
			Ω(fileEvent.File).Should(Equal(testFile))
		})
	})

	Context("Deleted", func() {
		var (
			subscriber <-chan FileEvent
		)

		BeforeEach(func() {
			fileEventTrigger = NewFileEventTrigger(fakeLogger, fakeFS, "/*.aliases", time.Second*2)
		})

		It("deleted notifies file event subscriber", func() {
			subscriber = fileEventTrigger.Subscribe()
			fakeFS.WriteFile(testFile, []byte(fmt.Sprint(`{}`)))
			go fileEventTrigger.Start()
			var fileEvent FileEvent

			Eventually(subscriber, time.Second*3).Should(Receive(&fileEvent))
			Ω(fileEvent.Type).Should(Equal(Added))
			Ω(fileEvent.File).Should(Equal(testFile))

			fakeFS.RemoveAll(testFile)

			var fileEvent2 FileEvent
			Eventually(subscriber, time.Second*3).Should(Receive(&fileEvent2))
			Ω(fileEvent2.Type).Should(Equal(Deleted))
			Ω(fileEvent2.File).Should(Equal(testFile))
		})
	})
})
