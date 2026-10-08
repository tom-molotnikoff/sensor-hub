//go:build unix

package secrets

import (
	"fmt"
	"os"
	"syscall"
)

// chownLikeIfRoot gives f the owner and group of reference when running as
// root. A local command run with sudo would otherwise leave a key file only
// root can read.
func chownLikeIfRoot(f *os.File, reference string) error {
	if reference == "" || os.Geteuid() != 0 {
		return nil
	}
	info, err := os.Stat(reference)
	if err != nil {
		return err
	}
	owner, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("cannot read the owner of %s", reference)
	}
	return f.Chown(int(owner.Uid), int(owner.Gid))
}
