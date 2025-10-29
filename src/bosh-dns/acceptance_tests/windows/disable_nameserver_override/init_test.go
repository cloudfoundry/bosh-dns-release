//go:build windows

package disable_nameserver_override_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDisableNameserverOverride(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "acceptance/windows/disable_nameserver_override")
}
