package bpffs

import "path"

const (
	BpfFsPath     = "/sys/fs/bpf"
	GoRoutinesDir = "goroutines"
)

var (
	GoRoutinesMapDir = path.Join(BpfFsPath, GoRoutinesDir)
)
