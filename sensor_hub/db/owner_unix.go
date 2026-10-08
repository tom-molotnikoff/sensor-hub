//go:build unix

package database

import (
	"fmt"
	"os"
	"syscall"
)

func chownToDirectoryOwnerIfRoot(f *os.File, dir string) error {
	if os.Geteuid() != 0 {
		return nil
	}
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	owner, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("cannot read the owner of %s", dir)
	}
	return f.Chown(int(owner.Uid), int(owner.Gid))
}
