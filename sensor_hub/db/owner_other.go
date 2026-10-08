//go:build !unix

package database

import "os"

func chownToDirectoryOwnerIfRoot(*os.File, string) error { return nil }
