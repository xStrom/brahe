// Copyright 2016-2019 Kaur Kuut
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/blake2b"
)

// TODO: Add back strict non-haystack check (for excessive files in target directories that don't exist in source)

// TODO: On Windows detect MAX_PATH violations -- even though we could bypass them with UNC, it's explorer nightmare

func getFileLists(dirNames []string) [][]os.FileInfo {
	// Get the file list for this directory
	allFileInfos := make([][]os.FileInfo, len(dirNames))
	for idx, dirName := range dirNames {
		files, err := ioutil.ReadDir(dirName)
		if err != nil {
			writeToConsole("ReadDir failed: %v", err)
			panic("")
		}
		allFileInfos[idx] = append(allFileInfos[idx], files...)
	}
	return allFileInfos
}

func findGaps(cfg *Config, progressValue float64, dirNames []string) {
	gapFormat := cfg.gapOpts.GetFormat()

	// Get the file list for this directory
	allFileInfos := getFileLists(dirNames)

	fiCount := 0
	for i := range allFileInfos {
		fiCount += len(allFileInfos[i])
	}

	var progressChunk float64
	if fiCount > 0 {
		progressChunk = progressValue / float64(fiCount)
	}
	progressExtra := progressValue - progressChunk*float64(fiCount)

	for i := range allFileInfos {
		foundFiles := make(map[string]bool, cfg.gapOpts.end-cfg.gapOpts.begin)
		for seq := cfg.gapOpts.begin; seq <= cfg.gapOpts.end; seq++ {
			foundFiles[fmt.Sprintf(gapFormat, seq)] = false
		}

		for j := range allFileInfos[i] {
			name := allFileInfos[i][j].Name()
			fullName := filepath.Join(dirNames[i], name)
			isDir := allFileInfos[i][j].IsDir()

			if !isDir {
				if _, ok := foundFiles[name]; ok {
					foundFiles[name] = true
				}
			}

			stats.lock.Lock()
			stats.currentPath = fullName
			stats.progress += progressChunk
			stats.lock.Unlock()
		}

		for seq := cfg.gapOpts.begin; seq <= cfg.gapOpts.end; seq++ {
			name := fmt.Sprintf(gapFormat, seq)
			if !foundFiles[name] {
				writeToConsole("MISSING %s", name)
			}
		}
	}

	stats.lock.Lock()
	stats.currentPath = ""
	stats.progress += progressExtra
	stats.lock.Unlock()
}

func compareDir(cfg *Config, progressValue float64, dirNames []string) {
	// Get the file list for this directory
	allFileInfos := getFileLists(dirNames)

	// Make sure they match
	fiCount := len(allFileInfos[0])

	var progressChunk float64
	if fiCount > 0 {
		progressChunk = progressValue / float64(fiCount)
	}
	progressExtra := progressValue - progressChunk*float64(fiCount)
	// TODO: Divide it even more by all the entries?

	for i := 0; i < fiCount; i++ {
		name := allFileInfos[0][i].Name()
		fullName := filepath.Join(dirNames[0], name)
		isDir := allFileInfos[0][i].IsDir()

		if (isDir && cfg.ignoreSpecificDirs[fullName]) || (!isDir && cfg.ignoreFiles[name]) {
			stats.lock.Lock()
			stats.progress += progressChunk
			stats.lock.Unlock()
			continue
		}

		stats.lock.Lock()
		stats.currentPath = fullName
		stats.lock.Unlock()

		allNames := make([]string, 0, len(allFileInfos))
		allNames = append(allNames, fullName)
		for j := 1; j < len(allFileInfos); j++ {
			searchName := filepath.Join(dirNames[j], name)
			found, dirMismatch := false, false
			for k := 0; k < len(allFileInfos[j]); k++ {
				n := allFileInfos[j][k].Name()
				if n == name {
					if allFileInfos[j][k].IsDir() == isDir {
						found = true
						allNames = append(allNames, searchName)
					} else {
						dirMismatch = true
						if isDir {
							reportMismatch("EXPECTED DIR %v", searchName)
						} else {
							reportMismatch("EXPECTED FILE %v", searchName)
						}
					}
					break
				}
			}
			if !found && !dirMismatch {
				reportMismatch("MISSING %v", searchName)
			}
		}

		if len(allNames) > 1 {
			if isDir {
				compareDir(cfg, progressChunk, allNames)
				continue // Progress was already incremented by compareDir
			} else if !cfg.noData {
				// Compare file hashes
				hashes := make([][]byte, len(allNames))
				speeds := make([]float64, len(allNames))

				var wg sync.WaitGroup
				wg.Add(len(allNames))
				for idx, name := range allNames {
					go func(idx int, name string) {
						hashes[idx], speeds[idx] = hashFile(name)
						wg.Done()
					}(idx, name)
				}
				wg.Wait()

				hash := hashes[0]
				avgSpeed := speeds[0]
				for j := 1; j < len(hashes); j++ {
					if !bytes.Equal(hash, hashes[j]) {
						reportMismatch("WRONG HASH %v", allNames[j])
					}
					avgSpeed += speeds[j]
				}
				avgSpeed /= float64(len(speeds))

				//writeToConsole("OK %.4f MB/s %x %v\n", avgSpeed, hash, allNames[0])
			}
		}
		// Increment the progress
		stats.lock.Lock()
		stats.progress += progressChunk
		stats.lock.Unlock()
	}

	stats.lock.Lock()
	stats.currentPath = ""
	stats.progress += progressExtra
	stats.lock.Unlock()
}

// Returns hash, MB/s
func hashFile(name string) ([]byte, float64) {
	t1 := time.Now()
	totalBytes := 0

	h, err := blake2b.New256(nil)
	if err != nil {
		writeToConsole("Failed to create blake2b hash: %v", err)
		panic("")
	}

	f, err := os.Open(name)
	if err != nil {
		writeToConsole("Failed to open file: %v - %v\n", name, err)
		panic("")
	}
	defer f.Close()

	buff := make([]byte, 4194304) // 4 MiB
	for {
		n, err := f.Read(buff)
		totalBytes += n
		if err == io.EOF {
			break
		} else if err != nil {
			writeToConsole("Failed reading file: %v - %v\n", name, err)
			panic("")
		}
		h.Write(buff[:n])
	}

	result := h.Sum(nil)

	t2 := time.Now()
	dur := t2.Sub(t1)
	MBps := (float64(totalBytes) / 1000 / 1000) / dur.Seconds()

	///writeToConsole("Hashed %v in %v - %v MB/s\n", name, dur, MBps)

	return result, MBps
}
