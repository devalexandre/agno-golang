//go:build !unix

package sandbox

import "os/exec"

func setProcessGroup(*exec.Cmd) {}
