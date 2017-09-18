package healthexecutable_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestHealthExecutable(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "healthcheck/healthexecutable")
}
