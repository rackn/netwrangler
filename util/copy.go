package util

import (
	"io"
	"os"
	"path"
	"path/filepath"
)

// Copy all of the files in one directory to another
func Copy(src, target string, e *Err) {
	names, err := filepath.Glob(path.Join(src, "*"))
	if err != nil {
		e.Merge(err)
		return
	}
	if err := os.MkdirAll(target, 0755); err != nil {
		e.Merge(err)
		return
	}
	for _, name := range names {
		if st, err := os.Stat(name); err != nil || st.Size() == 0 || st.IsDir() {
			continue
		}
		src, srcErr := os.Open(name)
		destName := path.Join(target, path.Base(name))
		dest, destErr := os.Create(destName)
		if srcErr == nil {
			defer src.Close()
		} else {
			e.Errorf("Error opening src temp %s: %v", name, srcErr)
		}
		if destErr == nil {
			defer dest.Close()
		} else {
			e.Errorf("Error opening dest %s: %v", destName, destErr)
		}
		if destErr != nil || srcErr != nil {
			continue
		}
		if _, err := io.Copy(dest, src); err != nil {
			e.Errorf("Error copying %s to %s: %v", name, destName, err)
		}
	}
}
