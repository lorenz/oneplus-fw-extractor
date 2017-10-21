package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func abortOnWriteErr(err error) {
	if err != nil {
		fmt.Printf("Failed to write: %v", err)
		os.Exit(1)
	}
}

func passThrough(path string) bool {
	return strings.HasPrefix(path, "firmware-update") || strings.HasPrefix(path, "RADIO") || path == "META-INF/com/google/android/update-binary"
}

func scanEdifyExpr(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, ';'); i >= 0 {
		// We have a full newline-terminated line.
		return i + 1, data[0:i], nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}

func main() {
	var image string
	var targetPath string
	flag.StringVar(&image, "i", "", "OxygenOS ZIP path")
	flag.StringVar(&targetPath, "o", "", "Firmware package path")
	flag.Parse()
	file, err := zip.OpenReader(image)
	if err != nil {
		fmt.Printf("Failed to open %v: %v", image, err)
		os.Exit(1)
	}
	targetFile, err := os.Create(targetPath)
	if err != nil {
		fmt.Printf("Failed to create %v: %v", image, err)
		os.Exit(1)
	}

	target := zip.NewWriter(targetFile)

	for _, f := range file.File {
		if passThrough(f.Name) {
			r, err := f.Open()
			abortOnWriteErr(err)
			w, err := target.CreateHeader(&f.FileHeader)
			abortOnWriteErr(err)
			_, err = io.Copy(w, r)
			abortOnWriteErr(err)
		}
		if f.Name == "META-INF/com/google/android/updater-script" {
			r, err := f.Open()
			abortOnWriteErr(err)
			w, err := target.CreateHeader(&f.FileHeader)
			abortOnWriteErr(err)
			s := bufio.NewScanner(r)
			s.Split(scanEdifyExpr)
			_, err = w.Write([]byte(fmt.Sprintf("ui_print(\"Firmware package created by oneplus-fw-extractor from %v\");\n", filepath.Base(image))))
			abortOnWriteErr(err)
			for s.Scan() {
				token := s.Text()
				if strings.Contains(token, "ro.display.series") {
					continue
				}
				if strings.Contains(token, "/dev/block/bootdevice/by-name/boot") {
					continue
				}
				if strings.Contains(token, "/dev/block/bootdevice/by-name/system") {
					continue
				}
				_, err = w.Write([]byte(token))
				w.Write([]byte(";"))
				abortOnWriteErr(err)
			}
			abortOnWriteErr(s.Err())
		}
	}
	err = target.Close()
	abortOnWriteErr(err)
}
