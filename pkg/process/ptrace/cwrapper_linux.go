//go:build cgo

package ptrace

/*
#define _GNU_SOURCE
#include <sys/wait.h>
#include <sys/uio.h>
#include <errno.h>
*/
import "C"

func waitpid(pid int) int {
	return int(C.waitpid(C.int(pid), nil, C.__WALL))
}
