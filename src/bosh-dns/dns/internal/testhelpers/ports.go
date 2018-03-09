package testhelpers

import (
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
)

const basePort = 4567

var portIndex int32 = -1

func GetFreePort() (int, error) {
	maxPorts := 2000 / config.GinkgoConfig.ParallelTotal
	if portIndex > int32(maxPorts-1) {
		return 0, fmt.Errorf("Cannot find a free port to use")
	}
	return basePort + int(atomic.AddInt32(&portIndex, 1)) + maxPorts*config.GinkgoConfig.ParallelNode, nil
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
