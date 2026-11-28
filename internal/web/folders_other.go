//go:build !windows

package web

func folderRoots() ([]string, error) { return []string{"/"}, nil }
