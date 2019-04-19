package healthconfig_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestHealthconfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "healthconfig")
}
