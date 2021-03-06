// Copyright 2016-2020 Kaur Kuut
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
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// TODO: Add back strict non-haystack check (for excessive files in target directories that don't exist in source)

// TODO: On Windows detect MAX_PATH violations -- even though we could bypass them with UNC, it's explorer nightmare

func splitProgressValue(value float64, parts int) (chunk float64, extra float64) {
	if parts > 0 {
		chunk = value / float64(parts)
	}
	extra = value - chunk*float64(parts)
	return
}

func getFileList(dirName string) []os.FileInfo {
	files, err := ioutil.ReadDir(dirName)
	if err != nil {
		writeToConsole("ReadDir failed: %v", err)
		panic("")
	}
	return files
}

func getFileLists(dirNames []string) [][]os.FileInfo {
	// Get the file list for this directory
	allFileInfos := make([][]os.FileInfo, len(dirNames))
	for idx, dirName := range dirNames {
		allFileInfos[idx] = getFileList(dirName)
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

	progressChunk, progressExtra := splitProgressValue(progressValue, fiCount)

	stats.lock.Lock()
	stats.missing = len(dirNames) * (cfg.gapOpts.end - cfg.gapOpts.begin + 1)
	stats.lock.Unlock()

	for i := range allFileInfos {
		foundFiles := make(map[string]bool, cfg.gapOpts.end-cfg.gapOpts.begin+1)
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
			if foundFiles[name] {
				stats.matched++
				stats.missing--
			}
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

const dbDirectory = "BraheDB"

func initDB(parentDir string) {
	dbDir := filepath.Join(parentDir, dbDirectory)
	if err := os.Mkdir(dbDir, 0666); err != nil && !os.IsExist(err) {
		writeToConsole("Failed to create directory %v: %v", dbDir, err)
		panic("")
	}
}

func verifyDB(parentDir string) {
	dbDir := filepath.Join(parentDir, dbDirectory)
	if fi, err := os.Stat(dbDir); err != nil {
		if os.IsNotExist(err) {
			writeToConsole("You need to build a database! No database exists in %v", parentDir)
			panic("")
		} else {
			writeToConsole("Failed to check database existance: %v", err)
			panic("")
		}
	} else if !fi.IsDir() {
		writeToConsole("The database needs to be inside a directory! %v is not a directory.", dbDir)
		panic("")
	}
}

// Returns true if any data was modified
func ensureDBEntry(parentDir string, hash []byte, entry string) bool {
	hashHex := fmt.Sprintf("%x", hash)
	hashFileDir := filepath.Join(parentDir, dbDirectory, hashHex[:2])
	if err := os.Mkdir(hashFileDir, 0666); err != nil && !os.IsExist(err) {
		writeToConsole("Failed to create directory %v: %v", hashFileDir, err)
		panic("")
	}
	hashFile := filepath.Join(hashFileDir, hashHex[2:])
	f, err := os.OpenFile(hashFile, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		writeToConsole("Failed to open file %v: %v", hashFile, err)
		panic("")
	}
	defer f.Close()
	b, err := ioutil.ReadAll(f)
	if err != nil {
		writeToConsole("Failed to read file %v: %v", hashFile, err)
		panic("")
	}
	lines := strings.Split(string(b), "\n")
	for _, line := range lines {
		if line == entry {
			return false
		}
	}
	if _, err := f.WriteString(entry + "\n"); err != nil {
		writeToConsole("Failed to add entry to file %v: %v", hashFile, err)
		panic("")
	}
	return true
}

func hasDBEntry(parentDir string, hash []byte) bool {
	hashHex := fmt.Sprintf("%x", hash)
	hashFile := filepath.Join(parentDir, dbDirectory, hashHex[:2], hashHex[2:])
	_, err := os.Stat(hashFile)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		writeToConsole("Failed to get file info %v: %v", hashFile, err)
		panic("")
	}
	return true
}

func useDB(cfg *Config, progressValue float64, dirName string, depth int) {
	fileInfos := getFileList(dirName)
	fiCount := len(fileInfos)

	progressChunk, progressExtra := splitProgressValue(progressValue, fiCount)

	for i := 0; i < fiCount; i++ {
		name := fileInfos[i].Name()
		fullName := filepath.Join(dirName, name)
		isDir := fileInfos[i].IsDir()

		if (isDir && cfg.ignoreSpecificDirs[fullName]) || (!isDir && cfg.ignoreFiles[name]) {
			stats.lock.Lock()
			stats.progress += progressChunk
			stats.ignored++
			stats.lock.Unlock()
			continue
		}

		stats.lock.Lock()
		stats.currentPath = fullName
		stats.lock.Unlock()

		var deltaMatched, deltaMissing, deltaCopied int
		if isDir {
			if depth != 0 {
				useDB(cfg, progressChunk, fullName, depth-1)
				continue // Progress was already incremented
			}
		} else {
			// Compare file hashes
			hash, _ := hashFile(fullName)
			//writeToConsole("OK %.4f MB/s %x %v", speed, hash, fullName)

			if cfg.buildDB {
				// Write out the DB entry
				if ensureDBEntry(cfg.entries[1], hash, fullName) {
					deltaCopied++
				} else {
					deltaMatched++
				}
			} else if cfg.checkDB {
				// Check if the DB entry exists
				if !hasDBEntry(cfg.entries[0], hash) {
					// Copy it if requested
					if len(cfg.copy) > 0 {
						// TODO: Rewrite the function to keep track of either the base entry or something like that,
						//       instead of this scanning to figure it out.
						suffix := ""
						for _, entry := range cfg.entries {
							if strings.HasPrefix(fullName, entry) {
								suffix = fullName[len(entry):]
								break
							}
						}
						if suffix == "" {
							panic("Didn't find entry prefix!")
						}
						dst := filepath.Join(cfg.copy, suffix)
						if err := os.MkdirAll(filepath.Dir(dst), 0666); err != nil {
							writeToConsole("Failed to create directory %v because: %v", filepath.Dir(dst), err)
							panic("")
						}
						if err := copyFile(fullName, dst); err != nil {
							writeToConsole("Failed to copy %v to %v because: %v", fullName, dst, err)
							panic("")
						}
						reportMismatch("COPIED %v", fullName)
						deltaCopied++
					} else {
						reportMismatch("MISSING %v", fullName)
						deltaMissing++
					}
				} else {
					deltaMatched++
				}
			}
		}

		// Increment the progress
		stats.lock.Lock()
		stats.progress += progressChunk
		stats.matched += deltaMatched
		stats.missing += deltaMissing
		stats.copied += deltaCopied
		stats.lock.Unlock()
	}

	stats.lock.Lock()
	stats.currentPath = ""
	stats.progress += progressExtra
	stats.lock.Unlock()
}

func compareDir(cfg *Config, progressValue float64, dirNames []string, depth int) {
	// Get the file list for this directory
	allFileInfos := getFileLists(dirNames)

	// Make sure they match
	fiCount := len(allFileInfos[0])

	progressChunk, progressExtra := splitProgressValue(progressValue, fiCount)

	for i := 0; i < fiCount; i++ {
		name := allFileInfos[0][i].Name()
		fullName := filepath.Join(dirNames[0], name)
		isDir := allFileInfos[0][i].IsDir()

		if (isDir && cfg.ignoreSpecificDirs[fullName]) || (!isDir && cfg.ignoreFiles[name]) {
			stats.lock.Lock()
			stats.progress += progressChunk
			stats.ignored++
			stats.lock.Unlock()
			continue
		}

		stats.lock.Lock()
		stats.currentPath = fullName
		stats.lock.Unlock()

		var deltaMatched, deltaMismatched, deltaMissing int

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
						deltaMatched++
						allNames = append(allNames, searchName)
					} else {
						dirMismatch = true
						deltaMismatched++
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
				deltaMissing++
				reportMismatch("MISSING %v", searchName)
			}
		}

		if len(allNames) > 1 {
			if isDir {
				if depth != 0 {
					compareDir(cfg, progressChunk, allNames, depth-1)
					stats.lock.Lock()
					stats.matched += deltaMatched
					stats.mismatched += deltaMismatched
					stats.missing += deltaMissing
					stats.lock.Unlock()
					continue // Progress was already incremented by compareDir
				}
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
						deltaMatched--
						deltaMismatched++
						reportMismatch("WRONG HASH %v", allNames[j])
					}
					avgSpeed += speeds[j]
				}
				avgSpeed /= float64(len(speeds))

				//writeToConsole("OK %.4f MB/s %x %v", avgSpeed, hash, allNames[0])
			}
		}
		// Increment the progress
		stats.lock.Lock()
		stats.progress += progressChunk
		stats.matched += deltaMatched
		stats.mismatched += deltaMismatched
		stats.missing += deltaMissing
		stats.lock.Unlock()
	}

	stats.lock.Lock()
	stats.currentPath = ""
	stats.progress += progressExtra
	stats.lock.Unlock()
}

func deleteDupes(cfg *Config, progressValue float64, dirName string, depth int, hashes map[[32]byte]struct{}) {
	fileInfos := getFileList(dirName)
	fiCount := len(fileInfos)

	progressChunk, progressExtra := splitProgressValue(progressValue, fiCount)

	for i := 0; i < fiCount; i++ {
		name := fileInfos[i].Name()
		fullName := filepath.Join(dirName, name)
		isDir := fileInfos[i].IsDir()

		if (isDir && cfg.ignoreSpecificDirs[fullName]) || (!isDir && cfg.ignoreFiles[name]) {
			stats.lock.Lock()
			stats.progress += progressChunk
			stats.ignored++
			stats.lock.Unlock()
			continue
		}

		stats.lock.Lock()
		stats.currentPath = fullName
		stats.lock.Unlock()

		var deltaMatched, deltaMismatched int
		if isDir {
			if depth != 0 {
				deleteDupes(cfg, progressChunk, fullName, depth-1, hashes)
				continue // Progress was already incremented
			}
		} else {
			// Compare file hashes
			hash, _ := hashFile(fullName)
			//writeToConsole("OK %.4f MB/s %x %v", speed, hash, fullName)

			var h [32]byte
			copy(h[:], hash[:])

			if _, ok := hashes[h]; ok {
				// Duplicate
				deltaMatched++
				//reportMismatch("DUPE %v", fullName)
				if err := os.Remove(fullName); err != nil {
					writeToConsole("Failed to delete %v because: %v", fullName, err)
					panic("")
				}
			} else {
				// New entry
				deltaMismatched++
				hashes[h] = struct{}{}
			}
		}

		// Increment the progress
		stats.lock.Lock()
		stats.progress += progressChunk
		stats.matched += deltaMatched
		stats.mismatched += deltaMismatched
		stats.lock.Unlock()
	}

	stats.lock.Lock()
	stats.currentPath = ""
	stats.progress += progressExtra
	stats.lock.Unlock()
}
