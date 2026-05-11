//go:build !linux

package system

// Stub untuk dev di non-Linux. Stats memory hanya tersedia di produksi (Linux).
func readMem() (total, free uint64) { return 0, 0 }
