package healthiness_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestHealthiness(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "dns/server/healthiness")
}
