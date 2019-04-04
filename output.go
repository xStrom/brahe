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
	"unicode/utf8"
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
	line string
	show bool
}

func (di *DisplayInfo) Show() {
	di.lock.Lock()
	di.show = true
	di.lock.Unlock()
}

func (di *DisplayInfo) Hide() {
	di.lock.Lock()
	if di.show {
		fmt.Print("\r")
		fmt.Print(getSpaces(maxLineWidth - 1))
		fmt.Print("\r")
	}
	di.show = false
	di.lock.Unlock()
}

type Stats struct {
	lock        sync.Mutex
	progress    float64
	currentPath string
	matched     int
	mismatched  int
	missing     int
	ignored     int
}

func (s *Stats) Clone() *Stats {
	return &Stats{
		progress:    s.progress,
		currentPath: s.currentPath,
		matched:     s.matched,
		mismatched:  s.mismatched,
		missing:     s.missing,
		ignored:     s.ignored,
	}
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

const maxLineWidth = 120 // TODO: Make this dynamic

func ensureLineWidths(data string) string {
	if strings.HasSuffix(data, "\n") {
		data = data[:len(data)-1]
	}
	newData := ""
	dataLines := strings.Split(data, "\n")
	for _, line := range dataLines {
		spaceCount := maxLineWidth - utf8.RuneCountInString(line) - 1
		newData += line + getSpaces(spaceCount) + "\n"
	}
	return newData
}

func writeToConsole(format string, a ...interface{}) {
	msg := time.Now().Format("[15:04:05] ") + fmt.Sprintf(format, a...)
	msg = ensureLineWidths(msg)

	displayInfo.lock.RLock()
	if displayInfo.show {
		fmt.Print("\r")
	}
	fmt.Print(msg)
	if displayInfo.show {
		fmt.Print(displayInfo.line)
	}
	displayInfo.lock.RUnlock()
}

func reportMismatch(format string, a ...interface{}) {
	writeToConsole(format, a...)
}

func setDisplayInfo(line string) {
	line = ensureLineWidths(line)
	line = line[:len(line)-1]

	displayInfo.lock.Lock()
	displayInfo.line = line
	if displayInfo.show {
		fmt.Print("\r")
		fmt.Print(displayInfo.line)
	}
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
			stats.lock.Lock()
			writeToConsole("Completed in %v with %d matches, %d mismatches, %d missing, %d ignored.", totalDurStr(), stats.matched, stats.mismatched, stats.missing, stats.ignored)
			stats.lock.Unlock()
			shutdown.wg.Done()
			return
		}
		shutdown.lock.RUnlock()

		time.Sleep(refreshRate)

		stats.lock.Lock()
		sc := stats.Clone()
		stats.lock.Unlock()

		line := fmt.Sprintf("[%v] [%.2f%% %d√ %dD %dM %dI] ", totalDurStr(), sc.progress, sc.matched, sc.mismatched, sc.missing, sc.ignored)
		path := sc.currentPath
		maxPathLen := maxLineWidth - utf8.RuneCountInString(line) - 1
		if utf8.RuneCountInString(path) > maxPathLen {
			path = path[utf8.RuneCountInString(path)-maxPathLen:]
		}
		line += path

		setDisplayInfo(line)
	}
}
