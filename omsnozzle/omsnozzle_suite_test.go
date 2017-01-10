package omsnozzle_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestOmsnozzle(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Omsnozzle Suite")
}
