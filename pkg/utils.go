package main

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

func readLinesFromFile(path string) ([]string, error) {
	var lines []string
	file, err := os.Open(path)
	if err != nil {
		return lines, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return lines, err
	}
	return lines, nil
}

func cleanPath(path string) string {
	newPath := filepath.Clean(path)
	volumeName := filepath.VolumeName(newPath)
	if volumeName != "" {
		newPath = strings.TrimLeft(newPath, volumeName)
	}
	newPath = filepath.ToSlash(newPath)
	newPath = strings.TrimLeft(newPath, "/")
	return newPath
}

func processWg(wg *sync.WaitGroup, finished chan bool, errs chan error) error {
	wg.Wait()
	close(finished)

	select {
	case <-finished:
	case err := <-errs:
		close(errs)
		return err
	}
	return nil
}

type WriteFakeCloser struct {
	io.Writer
}

func (rfc WriteFakeCloser) Close() error {
	return nil
}