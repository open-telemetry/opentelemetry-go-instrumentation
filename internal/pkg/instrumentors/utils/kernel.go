package utils

import (
	"strings"
	"syscall"

	"github.com/hashicorp/go-version"
)

func GetLinuxKernelVersion() (*version.Version, error) {
	var utsname syscall.Utsname

	if err := syscall.Uname(&utsname); err != nil {
		return nil, err
	}

	var buf [65]byte
	for i, v := range utsname.Release {
		buf[i] = byte(v)
	}

	ver := string(buf[:])
	if strings.Contains(ver, "-") {
		ver = strings.Split(ver, "-")[0]
	}

	return version.NewVersion(ver)
}