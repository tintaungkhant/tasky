//go:build unix

package git

import (
	"fmt"
	"golang.org/x/sys/unix"
)

func directoryIdentity(path string) (string, error) {
	var info unix.Stat_t
	if err := unix.Stat(path, &info); err != nil {
		return "", err
	}
	return fmt.Sprintf("unix:%x:%x", info.Dev, info.Ino), nil
}
