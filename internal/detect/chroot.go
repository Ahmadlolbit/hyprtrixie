package detect

import "os"

func Chroot() bool {
	root, err := os.Stat(at("/"))
	if err != nil {
		return false
	}
	first, err := os.Stat(at("/proc/1/root"))
	return err == nil && !os.SameFile(root, first)
}
