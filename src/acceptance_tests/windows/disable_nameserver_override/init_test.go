// +build windows

package disable_nameserver_override_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestDisableNameserverOverride(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "acceptance/windows/disable_nameserver_override")
}
