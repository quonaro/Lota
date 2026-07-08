//go:build linux

package runner

import (
	"syscall"
)

// getTermiosIOCTL returns the ioctl constant for getting terminal attributes.
func getTermiosIOCTL() uintptr {
	return syscall.TCGETS
}

// setTermiosIOCTL returns the ioctl constant for setting terminal attributes.
func setTermiosIOCTL() uintptr {
	return syscall.TCSETS
}
