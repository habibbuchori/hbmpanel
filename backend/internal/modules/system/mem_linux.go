//go:build linux

package system

import "syscall"

func readMem() (total, free uint64) {
	var info syscall.Sysinfo_t
	if syscall.Sysinfo(&info) != nil {
		return 0, 0
	}
	return info.Totalram * uint64(info.Unit), info.Freeram * uint64(info.Unit)
}

func readDisk(path string) (total, free uint64) {
	var st syscall.Statfs_t
	if syscall.Statfs(path, &st) != nil {
		return 0, 0
	}
	return st.Blocks * uint64(st.Bsize), st.Bavail * uint64(st.Bsize)
}
