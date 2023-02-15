package healthexecutable_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestHealthExecutable(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "healthcheck/healthexecutable")
}
