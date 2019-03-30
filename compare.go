// Copyright 2016 Kaur Kuut
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
	"crypto/sha1"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var ignoreNames = map[string]bool{}

func main() {
	entries := []string{"F:\\", "E:\\B\\"}
	for _, entry := range entries {
		ignoreNames[filepath.Join(entry, "$RECYCLE.BIN")] = true
		ignoreNames[filepath.Join(entry, "System Volume Information")] = true
	}
	compareDir(entries, false)
	fmt.Printf("OK All done! :)\n")
}

// TODO: Add back haystack option and make it work -- but differently in that, error on extra files in target dirs if the option isn't set

// TODO: Add command line parameter parsing for paths and options

// TODO: Support multiple paths, as [options] [truth] [target1] ... [targetN]

// TODO: On Windows detect MAX_PATH violations -- even though we could bypass them with UNC, it's explorer nightmare

func compareDir(dirNames []string, checkHash bool) {
	// Get the file list for this directory
	allFileInfos := make([][]os.FileInfo, len(dirNames))
	for idx, dirName := range dirNames {
		files, err := ioutil.ReadDir(dirName)
		if err != nil {
			fmt.Printf("ReadDir failed: %v\n", err)
			panic("")
		}
		allFileInfos[idx] = append(allFileInfos[idx], files...)
	}

	// Make sure they match
	fiCount := len(allFileInfos[0])

	for i := 0; i < fiCount; i++ {
		name := allFileInfos[0][i].Name()
		fullName := filepath.Join(dirNames[0], name)
		if ignoreNames[fullName] {
			continue
		}
		isDir := allFileInfos[0][i].IsDir()
		allNames := make([]string, 0, len(allFileInfos))
		allNames = append(allNames, fullName)
		for j := 1; j < len(allFileInfos); j++ {
			searchName := filepath.Join(dirNames[j], name)
			found := false
			for k := 0; k < len(allFileInfos[j]); k++ {
				n := allFileInfos[j][k].Name()
				if n == name {
					found = true
					if allFileInfos[j][k].IsDir() != isDir {
						fmt.Printf("Failed dir check! %v - %v - %v - %v - %v\n", j, i, k, isDir, allFileInfos[j][k].IsDir())
						panic("")
					}
					allNames = append(allNames, searchName)
					break
				}
			}
			if !found {
				fmt.Printf("Failed to locate! %v\n", searchName)
				panic("")
			}
		}

		if isDir {
			compareDir(allNames, checkHash)
		} else if checkHash {
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
					fmt.Printf("Hash wrong for file: %v - Expected %x - Got %x\n", allNames[j], hash, hashes[j])
					panic("")
				}
				avgSpeed += speeds[j]
			}
			avgSpeed /= float64(len(speeds))

			fmt.Printf("OK %.4f MB/s %x %v\n", avgSpeed, hash, allNames[0])
		}
	}
}

// Returns hash, MB/s
func hashFile(name string) ([]byte, float64) {
	t1 := time.Now()
	totalBytes := 0

	h := sha1.New()

	f, err := os.Open(name)
	if err != nil {
		fmt.Printf("Failed to open file: %v - %v\n", name, err)
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
			fmt.Printf("Failed reading file: %v - %v\n", name, err)
			panic("")
		}
		h.Write(buff[:n])
	}

	result := h.Sum(nil)

	t2 := time.Now()
	dur := t2.Sub(t1)
	MBps := (float64(totalBytes) / 1000 / 1000) / dur.Seconds()

	///fmt.Printf("Hashed %v in %v - %v MB/s\n", name, dur, MBps)

	return result, MBps
}
