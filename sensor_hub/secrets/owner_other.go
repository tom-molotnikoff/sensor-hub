//go:build !unix

package secrets

import "os"

func chownLikeIfRoot(*os.File, string) error { return nil }
