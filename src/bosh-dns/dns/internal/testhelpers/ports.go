package testhelpers

import (
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega" //nolint:staticcheck
)

const basePort = 4567

var portIndex int32 = -1

func GetFreePort() (int, error) {
	suite, _ := ginkgo.GinkgoConfiguration()
	maxPorts := 2000 / suite.ParallelTotal
	for {
		if portIndex > int32(maxPorts-1) { //nolint:staticcheck
			break
		}
		unusedport := basePort + int(atomic.AddInt32(&portIndex, 1)) + maxPorts*suite.ParallelProcess
		err := TryListening(unusedport)
		if err == nil {
			return unusedport, nil
		}
	}
	return 0, fmt.Errorf("Cannot find a free port to use") //nolint:staticcheck
}

func WaitForListeningTCP(port int) error {
	for i := 0; i < 20; i++ {
		c, err := net.Dial("tcp", fmt.Sprintf("%s:%d", "127.0.0.1", port))
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		err = c.Close()
		Expect(err).NotTo(HaveOccurred())
		return nil
	}

	return errors.New("dns server failed to start")
}

func TryListening(port int) error {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return err
	}
	err = ln.Close()
	return err
}
