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
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

type Shutdown struct {
	wg    sync.WaitGroup
	lock  sync.RWMutex
	start bool
}

func (s *Shutdown) AddWorkers(delta int) {
	s.wg.Add(delta)
}

func (s *Shutdown) Wait() {
	s.wg.Wait()
}

func (s *Shutdown) Start() {
	s.lock.Lock()
	s.start = true
	s.lock.Unlock()
}

type DisplayInfo struct {
	lock sync.RWMutex
	info string
	show bool
}

func (di *DisplayInfo) Show() {
	di.lock.Lock()
	di.show = true
	di.lock.Unlock()
}

func (di *DisplayInfo) Hide() {
	di.lock.Lock()
	di.show = false
	di.lock.Unlock()
}

type Stats struct {
	lock        sync.Mutex
	progress    float64
	currentPath string
}

var (
	displayInfo = DisplayInfo{}
	shutdown    = Shutdown{}
	stats       = Stats{}
)

func getSpaces(count int) string {
	if count <= 0 {
		return ""
	}
	return strings.Repeat(" ", count)
}

const maxLineWidth = 119 // TODO: Make this dynamic

func ensureLineWidths(data string) string {
	if strings.HasSuffix(data, "\n") {
		data = data[:len(data)-1]
	}
	newData := ""
	dataLines := strings.Split(data, "\n")
	for _, line := range dataLines {
		spaceCount := maxLineWidth - len(line) - (strings.Count(line, "\t") * 6)
		newData += line + getSpaces(spaceCount) + "\n"
	}
	return newData
}

func writeToConsole(format string, a ...interface{}) {
	info := time.Now().Format("[15:04:05] ") + fmt.Sprintf(format, a...)
	info = ensureLineWidths(info)

	fmt.Print("\r")
	fmt.Print(info)

	displayInfo.lock.RLock()
	if displayInfo.show {
		fmt.Print("\r")
		fmt.Print(displayInfo.info)
	}
	displayInfo.lock.RUnlock()
}

func reportMismatch(format string, a ...interface{}) {
	writeToConsole(format, a...)
}

func displayProgress(info string) {
	info = ensureLineWidths(info)
	info = strings.Replace(info, "\n", "", -1)

	fmt.Print("\r")
	fmt.Print(info)

	displayInfo.lock.Lock()
	displayInfo.info = info
	displayInfo.lock.Unlock()
}

func statsGalore() {
	refreshRate := time.Millisecond * 100
	totalStart := time.Now()

	totalDurStr := func() string {
		totalDur := time.Since(totalStart)
		totalDurH := uint32(math.Floor(totalDur.Hours()))
		totalDurM := uint32(math.Floor(totalDur.Minutes())) % 60
		totalDurS := uint32(math.Floor(totalDur.Seconds())) % 60
		return fmt.Sprintf("%02d:%02d:%02d", totalDurH, totalDurM, totalDurS)
	}

	for {
		// Is there a shut down sequence?
		shutdown.lock.RLock()
		if shutdown.start {
			shutdown.lock.RUnlock()
			writeToConsole("Completed in %v", totalDurStr())
			shutdown.wg.Done()
			return
		}
		shutdown.lock.RUnlock()

		time.Sleep(refreshRate)

		statsCache := Stats{}

		stats.lock.Lock()
		// Cache the stats
		statsCache.progress = stats.progress
		statsCache.currentPath = stats.currentPath
		stats.lock.Unlock()

		info := fmt.Sprintf("[Time %v] [%.2f%%] ", totalDurStr(), statsCache.progress)
		path := statsCache.currentPath
		maxPathLen := maxLineWidth - len(info)
		if len(path) > maxPathLen {
			path = path[len(path)-maxPathLen:]
		}

		displayProgress(info + path)
	}
}
