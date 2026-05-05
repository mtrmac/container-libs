package machine

import (
	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = Describe("Machine", func() {
	It("not a machine", func() {
		marker := loadMachineMarker("testdata/does-not-exist", "testdata/does-not-exist")

		gomega.Expect(marker.IsPodmanMachine()).To(gomega.BeFalse())
		gomega.Expect(marker.HostType()).To(gomega.BeEmpty())
		gomega.Expect(marker.IsGvProxyBased()).To(gomega.BeFalse())
	})

	It("generic machine", func() {
		marker := loadMachineMarker("testdata/empty-machine", "testdata/does-not-exist")

		gomega.Expect(marker.IsPodmanMachine()).To(gomega.BeTrue())
		gomega.Expect(marker.HostType()).To(gomega.BeEmpty())
		gomega.Expect(marker.IsGvProxyBased()).To(gomega.BeTrue())
	})

	It("wsl machine", func() {
		marker := loadMachineMarker("testdata/wsl-machine", "testdata/does-not-exist")

		gomega.Expect(marker.IsPodmanMachine()).To(gomega.BeTrue())
		gomega.Expect(marker.HostType()).To(gomega.Equal(Wsl))
		gomega.Expect(marker.IsGvProxyBased()).To(gomega.BeFalse())
	})

	It("qemu machine", func() {
		marker := loadMachineMarker("testdata/qemu-machine", "testdata/does-not-exist")

		gomega.Expect(marker.IsPodmanMachine()).To(gomega.BeTrue())
		gomega.Expect(marker.HostType()).To(gomega.Equal(Qemu))
		gomega.Expect(marker.IsGvProxyBased()).To(gomega.BeTrue())
	})

	It("applehv machine", func() {
		marker := loadMachineMarker("testdata/applehv-machine", "testdata/does-not-exist")

		gomega.Expect(marker.IsPodmanMachine()).To(gomega.BeTrue())
		gomega.Expect(marker.HostType()).To(gomega.Equal(AppleHV))
		gomega.Expect(marker.IsGvProxyBased()).To(gomega.BeTrue())
	})

	It("hyperv machine", func() {
		marker := loadMachineMarker("testdata/hyperv-machine", "testdata/does-not-exist")

		gomega.Expect(marker.IsPodmanMachine()).To(gomega.BeTrue())
		gomega.Expect(marker.HostType()).To(gomega.Equal(HyperV))
		gomega.Expect(marker.IsGvProxyBased()).To(gomega.BeTrue())
	})

	It("fallback path", func() {
		marker := loadMachineMarker("testdata/does-not-exist", "testdata/hyperv-machine")

		gomega.Expect(marker.IsPodmanMachine()).To(gomega.BeTrue())
		gomega.Expect(marker.HostType()).To(gomega.Equal(HyperV))
		gomega.Expect(marker.IsGvProxyBased()).To(gomega.BeTrue())
	})
})
