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
	"io"
	"os"
	"time"

	"golang.org/x/crypto/blake2b"
)

// TODO: Improve the function to:
//       #1 Copy also metadata like time created & time modified & access lists & possibly alternate streams
//       #2 Copy it in chunks to be able to report copying speed to the stats engine
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0666)
	if err != nil {
		return err
	}
	defer out.Close() // Defer it to be sure it's closed, althoguh we'll manually close it in a good scenario

	if _, err = io.Copy(out, in); err != nil {
		return err
	}

	if err = out.Sync(); err != nil {
		return err
	}

	return out.Close()
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
		writeToConsole("Failed to open file: %v - %v", name, err)
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
			writeToConsole("Failed reading file: %v - %v", name, err)
			panic("")
		}
		h.Write(buff[:n])
	}

	result := h.Sum(nil)

	t2 := time.Now()
	dur := t2.Sub(t1)
	MBps := (float64(totalBytes) / 1000 / 1000) / dur.Seconds()

	///writeToConsole("Hashed %v in %v - %v MB/s", name, dur, MBps)

	return result, MBps
}
