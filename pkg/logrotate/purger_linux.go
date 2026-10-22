//go:build linux
// +build linux

package logrotate

import "syscall"

// getDiskUsagePct returns the disk usage percentage of the filesystem containing path.
func getDiskUsagePct(path string) (int, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	if total == 0 {
		return 0, nil
	}
	used := total - free
	return int((float64(used) / float64(total)) * 100), nil
}
