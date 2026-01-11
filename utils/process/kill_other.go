//go:build !windows

package process

import (
	"errors"

	gopsprocess "github.com/shirou/gopsutil/v4/process"
)

const maxKillTreeDepth = 10

// KillProcessTree best-effort terminates the given process and its descendants.
// It returns nil when the root process doesn't exist.
func KillProcessTree(pid int32) error {
	if pid <= 0 {
		return nil
	}

	root, err := gopsprocess.NewProcess(pid)
	if err != nil {
		return nil
	}

	visited := map[int32]struct{}{pid: {}}
	var descendants []int32
	collectDescendants(root, 0, maxKillTreeDepth, visited, &descendants)

	var errs []error
	// Kill descendants first to avoid re-parenting surprises.
	for _, childPID := range descendants {
		errs = append(errs, killSingle(childPID))
	}
	errs = append(errs, killSingle(pid))

	return errors.Join(errs...)
}

func collectDescendants(proc *gopsprocess.Process, depth, maxDepth int, visited map[int32]struct{}, out *[]int32) {
	if proc == nil || depth >= maxDepth {
		return
	}

	children, err := proc.Children()
	if err != nil || len(children) == 0 {
		return
	}

	for _, child := range children {
		if child == nil {
			continue
		}
		childPID := child.Pid
		if childPID <= 0 {
			continue
		}
		if _, ok := visited[childPID]; ok {
			continue
		}
		visited[childPID] = struct{}{}
		collectDescendants(child, depth+1, maxDepth, visited, out)
		// Post-order so that deeper descendants are killed first.
		*out = append(*out, childPID)
	}
}

func killSingle(pid int32) error {
	if pid <= 0 {
		return nil
	}
	proc, err := gopsprocess.NewProcess(pid)
	if err != nil {
		return nil
	}
	// Terminate is a softer attempt; fall back to Kill when unsupported or rejected.
	if err := proc.Terminate(); err == nil {
		return nil
	}
	if err := proc.Kill(); err == nil {
		return nil
	}
	return err
}
