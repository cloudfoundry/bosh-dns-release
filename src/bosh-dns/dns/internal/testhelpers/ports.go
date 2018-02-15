package testhelpers

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	. "github.com/onsi/gomega"
)

func GetFreePort() (int, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	l.Close()

	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		return 0, err
	}

	intPort, err := strconv.Atoi(port)
	if err != nil {
		return 0, err
	}

	return intPort, nil
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
