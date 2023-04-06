package healthexecutable_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestHealthExecutable(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "healthcheck/healthexecutable")
}
