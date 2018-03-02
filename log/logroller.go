// Copyright 2015 Google Inc. All rights reserved.

// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd

// +build linux darwin freebsd

package log

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"sync"
)

var rollPattern = regexp.MustCompile(`^\.([0-9]+)$`)

const rollFormatFormat = "%s.%%0%dd"

type LogRoller struct {
	fileName     string
	fileMaxBytes uint
	maxFiles     uint

	rollFormat string

	m        sync.Mutex
	file     *os.File
	fileSize uint
}

func NewLogRoller(fileName string, fileMaxBytes, maxFiles uint) (*LogRoller, error) {
	// How many digits to append to rolled file name?
	// 0 => 0 ; 1 => 1 ; 9 => 1 ; 99 => 2 ; 100 => 3
	var digits int
	if maxFiles > 0 {
		digits = int(math.Log10(float64(maxFiles))) + 1
	}
	rollFormat := fmt.Sprintf(rollFormatFormat, fileName, digits)

	lr := LogRoller{
		fileName:     fileName,
		fileMaxBytes: fileMaxBytes,
		maxFiles:     maxFiles,
		rollFormat:   rollFormat,
	}
	return &lr, nil
}

func (lr *LogRoller) Write(p []byte) (int, error) {
	lr.m.Lock()
	defer lr.m.Unlock()

	if lr.file == nil {
		lr.fileSize = 0
		if err := lr.openFile(); err != nil {
			return 0, err
		}
	}

	written, err := lr.file.Write(p)
	if err != nil {
		return 0, err
	}

	lr.fileSize += uint(written)
	if lr.fileSize > lr.fileMaxBytes {
		lr.file.Close()
		lr.file = nil
	}

	return written, nil
}

// openFile opens a new file for logging, rolling the oldest one if needed.
func (lr *LogRoller) openFile() error {
	if err := lr.roll(); err != nil {
		return err
	}

	if f, err := os.Create(lr.fileName); err != nil {
		return err
	} else {
		lr.file = f
	}

	return nil
}

type sortableNumberStrings []string

func (s sortableNumberStrings) Len() int {
	return len(s)
}

func (s sortableNumberStrings) Less(i, j int) bool {
	ip, _ := strconv.ParseUint(s[i], 10, 16)
	jp, _ := strconv.ParseUint(s[j], 10, 16)
	return ip < jp
}

func (s sortableNumberStrings) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// roll deletes old log files until there are lr.maxFiles or fewer,
// and renames remaining log files so that the file named lr.fileName+".3" becomes lr.fileName+".4".
// The file named lr.fileName becomes lr.fileName+".0".
// If lr.fileName does not exist, then this is a noop.
func (lr *LogRoller) roll() error {
	if _, err := os.Stat(lr.fileName); os.IsNotExist(err) {
		// Nothing to do; the target log file name already does not exist.
		return nil
	}

	// Get all rolled logs, plus some.
	allFiles, err := filepath.Glob(lr.fileName + ".*")
	if err != nil {
		return err
	}

	// Get number suffixes from the rolled logs; ignore non-matches.
	numbers := make(sortableNumberStrings, 0, len(allFiles))
	for _, file := range allFiles {
		match := rollPattern.FindStringSubmatch(file[len(lr.fileName):])
		if len(match) < 2 {
			continue
		}
		if _, err := strconv.ParseUint(match[1], 10, 16); err == nil {
			// Keep the string form of the number.
			numbers = append(numbers, match[1])
		}
	}

	// Delete old log files and rename the rest.
	sort.Sort(numbers)
	for i := len(numbers) - 1; i >= 0; i-- {
		oldpath := fmt.Sprintf("%s.%s", lr.fileName, numbers[i])
		if uint(i+1) >= lr.maxFiles {
			err := os.Remove(oldpath)
			if err != nil {
				return err
			}

		} else {
			n, _ := strconv.ParseUint(numbers[i], 10, 16)
			newpath := fmt.Sprintf(lr.rollFormat, n+1)
			err := os.Rename(oldpath, newpath)
			if err != nil {
				return err
			}
		}
	}

	if lr.maxFiles > 0 {
		newpath := fmt.Sprintf(lr.rollFormat, 0)
		err = os.Rename(lr.fileName, newpath)
		if err != nil {
			return err
		}
	} // Else the existing file will be truncated.

	return nil
}
