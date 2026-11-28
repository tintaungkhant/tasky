package web

import "golang.org/x/sys/windows"

func folderRoots() ([]string, error) {
	drives, err := windows.GetLogicalDrives()
	if err != nil {
		return nil, err
	}
	roots := []string{}
	for i := uint(0); i < 26; i++ {
		if drives&(1<<i) != 0 {
			roots = append(roots, string(rune('A'+i))+`:\`)
		}
	}
	return roots, nil
}
