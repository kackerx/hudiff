//go:build windows

package agents

// Windows has no ps/lsof pair to ask; the page shows an empty list (spec §11.4).
func InRepo(repo string, extra []string) []Agent { return nil }

func Alive(pid int) bool { return false }
