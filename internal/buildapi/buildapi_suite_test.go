package buildapi

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBuildapi(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "BuildAPI Suite")
}
