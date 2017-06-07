package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"github.com/onsi/gomega/gexec"
	"os/exec"
	"fmt"
	"strconv"
	"time"
	"net"
	"errors"
	"io/ioutil"
	"encoding/json"
)

var (
	sess *gexec.Session
	cmd  *exec.Cmd
	listenPort int
)

var _ = Describe("HealthCheck server", func() {
	BeforeEach(func() {
		var err error

		// run the server
		listenPort = 8080
		cmd = exec.Command(pathToServer)
		sess, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())

		Expect(waitForServer(listenPort)).To(Succeed())
	})

	AfterEach(func() {
		if cmd.Process != nil {
			sess.Terminate()
			sess.Wait()
		}
	})

	Describe("/health", func() {
		It("returns healthy json output", func() {
			resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", listenPort)) // TODO configure addresses and ports
			Expect(err).ToNot(HaveOccurred())

			respData,err := ioutil.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())

			var respJson map[string]string
			err = json.Unmarshal(respData, &respJson)
			Expect(err).ToNot(HaveOccurred())

			Expect(respJson).To(Equal(map[string]string{
				"state": "running",
			}))
		})

		It("listens on all configured addresses", func() {

		})
	})
})

func waitForServer(port int) error {
	for i := 0; i < 20; i++ {
		c, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%s", strconv.Itoa(port)))
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		c.Close()
		return nil
	}

	return errors.New("dns server failed to start")
}
