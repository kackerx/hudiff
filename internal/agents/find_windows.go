//go:build windows

package agents

// Windows has no ps/lsof pair to ask; the page shows an empty list (spec §11.4).
func List(repo string, extra []string, waiters []int, boundPID int) []Agent { return nil }

func Alive(pid int) bool { return false }
