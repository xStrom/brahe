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
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type GapOpts struct {
	prefix string
	suffix string
	width  int
	begin  int
	end    int
}

func (gopt *GapOpts) GetFormat() string {
	return fmt.Sprintf("%s%%0%dd%s", gopt.prefix, gopt.width, gopt.suffix)
}

type gapOptsValue struct {
	gapOpts **GapOpts
}

func (gov *gapOptsValue) String() string {
	return ""
}

func (gov *gapOptsValue) Set(pattern string) error {
	pieces := strings.Split(pattern, "/")
	if len(pieces) != 3 {
		return fmt.Errorf("Expected two forward slashes.")
	}
	gapOpts := &GapOpts{prefix: pieces[0], suffix: pieces[2]}
	n, err := fmt.Sscanf(pieces[1], "%d:%d-%d", &gapOpts.width, &gapOpts.begin, &gapOpts.end)
	if err != nil {
		return fmt.Errorf("Failed to extract gap range: %v", err)
	}
	if n != 3 {
		return fmt.Errorf("Failed to extract gap range.")
	}
	*gov.gapOpts = gapOpts
	return nil
}

type Config struct {
	depth              int
	entries            []string
	noData             bool
	checkSysNames      bool
	ignoreSpecificDirs map[string]bool
	ignoreFiles        map[string]bool
	gapOpts            *GapOpts
	buildDB            bool
	checkDB            bool
	deleteDupes        bool
	copy               string
}

const AppName = "brahe"

func getConfig(arguments []string) (*Config, error) {
	cfg := &Config{}
	f := flag.NewFlagSet(AppName, flag.ContinueOnError)
	f.Var(
		&gapOptsValue{&cfg.gapOpts},
		"find-gaps",
		"The `pattern` 'IMG_/4:14-155/.JPG' searches for gaps in sequence of IMG_0014.JPG .. IMG_0155.JPG.\nPattern '/0:1-13/.txt' seeks 1.txt .. 13.txt.",
	)
	f.IntVar(
		&cfg.depth,
		"depth",
		-1,
		"Specify how deep into the directory hierarchy to look into.\nUse 0 to check only immediate files/directories with no traversing.\nUse -1 for no limit.",
	)
	f.BoolVar(
		&cfg.noData,
		"no-data",
		false,
		"Don't compare the file contents.",
	)
	f.BoolVar(
		&cfg.checkSysNames,
		"system-names",
		false,
		"Also check system names like $RECYCLE.BIN;System Volume Information;found.000;Thumbs.db.",
	)
	f.BoolVar(
		&cfg.buildDB,
		"build-db",
		false,
		"Builds a hash database of all entries in [source] to [target1].",
	)
	f.BoolVar(
		&cfg.checkDB,
		"check-db",
		false,
		"Checks all files in [target1] .. [targetN] against the hash database in [source].",
	)
	f.BoolVar(
		&cfg.deleteDupes,
		"delete-dupes",
		false,
		"Deletes any duplicate files in [source].",
	)
	f.StringVar(
		&cfg.copy,
		"copy",
		"",
		"Any files not found in the database with -check-db are copied into the provided `directory`.")
	f.Usage = func() {
		fmt.Fprintf(f.Output(), "Usage:\n\n%s [options] [source] [target1] .. [targetN]\n\n", AppName)
		f.PrintDefaults()
	}
	if err := f.Parse(arguments); err != nil {
		return nil, err
	}
	failf := func(format string, a ...interface{}) error {
		err := fmt.Errorf(format, a...)
		fmt.Fprintln(f.Output(), err)
		f.Usage()
		return err
	}
	if cfg.noData && (cfg.buildDB || cfg.checkDB) {
		return nil, failf("Can't deal with the hash database without looking at file contents! Check your options.")
	}
	minArgs := 2
	if cfg.gapOpts != nil || cfg.deleteDupes {
		minArgs = 1
	}
	args := f.Args()
	if argsLen := len(args); argsLen < minArgs {
		return nil, failf("Expected %d targets, got %d.", minArgs, argsLen)
	}
	for i := range args {
		entry, err := filepath.Abs(args[i])
		if err != nil {
			return nil, failf("Invalid path? %v - %v", args[i], err)
		}
		cfg.entries = append(cfg.entries, entry)
	}
	if !cfg.checkSysNames {
		cfg.ignoreSpecificDirs = map[string]bool{}
		for _, entry := range cfg.entries {
			cfg.ignoreSpecificDirs[filepath.Join(entry, "$RECYCLE.BIN")] = true
			cfg.ignoreSpecificDirs[filepath.Join(entry, "$Recycle.Bin")] = true
			cfg.ignoreSpecificDirs[filepath.Join(entry, "System Volume Information")] = true
			cfg.ignoreSpecificDirs[filepath.Join(entry, "found.000")] = true
		}
		cfg.ignoreFiles = map[string]bool{}
		cfg.ignoreFiles["Thumbs.db"] = true
	}
	return cfg, nil
}

// NOTE: Also returns false in case of EOF (e.g. Ctrl+C)
func askBool(question string) bool {
	fmt.Printf("%v (Y/N) - ", question)
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		answer := strings.ToLower(scanner.Text())
		if answer == "n" {
			return false
		} else if answer == "y" {
			return true
		}
		fmt.Printf("%v (Y/N) - ", question)
	}
	if err := scanner.Err(); err != nil {
		fmt.Printf("\nScan failed: %v\n", err)
	}
	return false
}

func main() {
	// Parse the command line arguments
	cfg, err := getConfig(os.Args[1:])
	if err != nil {
		os.Exit(2)
	}

	for i := range cfg.entries {
		header := "   Source"
		if i > 0 {
			header = fmt.Sprintf("Target #%d", i)
		}
		fmt.Printf("%v: %v\n", header, cfg.entries[i])
	}
	if !askBool("Start comparing?") {
		return
	}

	// NOTE: From here on out, we no longer directly use fmt.Printf
	writeToConsole("Starting work ..")
	displayInfo.Show()
	shutdown.AddWorkers(1)
	go statsGalore()

	if cfg.gapOpts != nil {
		findGaps(cfg, 100.0, cfg.entries)
	} else if cfg.deleteDupes {
		deleteDupes(cfg, 100.0, cfg.entries[0], cfg.depth, map[[32]byte]struct{}{})
	} else if cfg.buildDB {
		initDB(cfg.entries[1])
		useDB(cfg, 100.0, cfg.entries[0], cfg.depth)
	} else if cfg.checkDB {
		verifyDB(cfg.entries[0])
		progressChunk, progressExtra := splitProgressValue(100.0, len(cfg.entries)-1)
		for i := 1; i < len(cfg.entries); i++ {
			useDB(cfg, progressChunk, cfg.entries[i], cfg.depth)
		}
		stats.lock.Lock()
		stats.progress += progressExtra
		stats.lock.Unlock()
	} else {
		compareDir(cfg, 100.0, cfg.entries, cfg.depth)
	}

	displayInfo.Hide()
	shutdown.Start()
	shutdown.Wait()
}
