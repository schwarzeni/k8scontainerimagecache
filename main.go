package main

import (
	"fmt"
	"io/fs"
	"path/filepath"
)

func main() {
	filepath.Walk("/Users/nizhenyang/Desktop/论文 workspace/code/container-image-cache/", func(path string, info fs.FileInfo, err error) error {
		fmt.Println(path)
		return err
	})
}
