package utils

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
)

func TarDirectoryToReader(dir string) (io.Reader, error) {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	defer tw.Close()

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		header, _ := tar.FileInfoHeader(info, rel)
		header.Name = rel
		tw.WriteHeader(header)
		f, _ := os.Open(path)
		io.Copy(tw, f)
		f.Close()
		return nil
	})

	return bytes.NewReader(buf.Bytes()), nil
}
