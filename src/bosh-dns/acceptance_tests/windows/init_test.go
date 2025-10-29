//go:build windows

package windows_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestWindows(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "acceptance/windows")
}
