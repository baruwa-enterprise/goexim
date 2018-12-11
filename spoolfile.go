// Copyright (C) 2018 Andrew Colin Kissa <andrew@datopdog.io>
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at http://mozilla.org/MPL/2.0/.

/*
Package spoolfile Reads and parses Exim spool files
*/
package spoolfile

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"
	"syscall"
)

var (
	// HfRe header file regex
	HfRe = regexp.MustCompile(`^(?:[^\W_]{6}-){2}[^\W_]{2}-H$`)
	// DfRe data file regex
	DfRe = regexp.MustCompile(`^(?:[^\W_]{6}-){2}[^\W_]{2}-D$`)
	// MidRe message-id regex
	MidRe = regexp.MustCompile(`((?:[^\W_]{6}-){2}[^\W_]{2})$`)
)

const (
	pathErr = "The path: %s is not a directory"
	fileErr = "The path: %s is not a regular file"
)

func trim(s []byte) []byte {
	return bytes.TrimRight(s, "\n")
}

// Msg struct
type Msg struct {
	ID        []byte
	User      []byte
	UID       int
	GID       int
	Sender    []byte
	Received  int
	WarnCount int
	ACL       map[string][]byte
	Aclc      map[string][]byte
	Aclm      map[string][]byte
	DashVars  map[string][]byte
	NonRcpts  [][]byte
	NumRcpts  int
	Rcpts     [][]byte
	Hdrs      [][]byte
	RawHdrs   [][]byte
	HdrFile   string
	DtaFile   string
	mx        sync.Mutex
	hf        *os.File
	df        *os.File
}

// CreateFile creates the .eml representation on disk
func (m *Msg) CreateFile(fn string) (err error) {
	m.mx.Lock()
	defer m.mx.Unlock()
	var f *os.File
	var lineb []byte

	f, err = os.OpenFile(fn, syscall.O_CREAT|syscall.O_RDWR, 0640)
	if err != nil {
		return
	}
	defer f.Sync()
	defer f.Close()

	for _, h := range m.Hdrs {
		f.Write(h)
		f.WriteString("\n")
	}
	f.WriteString("\n")

	m.df.Seek(0, 0)
	dr := bufio.NewReader(m.df)

	_, err = dr.ReadBytes('\n')
	if err != nil {
		return
	}

	for {
		lineb, err = dr.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				err = nil
				break
			}
			return
		}
		f.Write(lineb)
	}
	return
}

func (m *Msg) String() (string, error) {
	m.mx.Lock()
	defer m.mx.Unlock()
	var err error
	var lineb []byte
	var b strings.Builder
	for _, h := range m.Hdrs {
		fmt.Fprintf(&b, "%s\n", h)
	}
	// Print newline separating headers from body
	fmt.Fprint(&b, "\n")

	m.df.Seek(0, 0)
	dr := bufio.NewReader(m.df)
	// Readin the message-id
	_, err = dr.ReadBytes('\n')
	if err != nil {
		return "", err
	}
	// Read the message body
	for {
		lineb, err = dr.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				err = nil
				break
			}
			return "", err
		}
		fmt.Fprintf(&b, "%s", lineb)
	}

	return b.String(), err
}

// Close frees the locks and closes the files
func (m *Msg) Close() {
	m.mx.Lock()
	defer m.mx.Unlock()
	dfLock := syscall.Flock_t{
		Type:   syscall.F_WRLCK,
		Start:  0,
		Len:    0,
		Whence: 0,
	}
	hfLock := syscall.Flock_t{
		Type:   syscall.F_WRLCK,
		Start:  0,
		Len:    0,
		Whence: 0,
	}
	syscall.FcntlFlock(m.df.Fd(), syscall.F_UNLCK, &dfLock)
	syscall.FcntlFlock(m.hf.Fd(), syscall.F_UNLCK, &hfLock)
	m.df.Close()
	m.hf.Close()
}

// NewMsg creates a new Msg
func NewMsg(p, id string) (m *Msg, err error) {
	var hp, dp, hid, did string
	if !MidRe.MatchString(id) {
		err = fmt.Errorf("Invalid exim id: %s", id)
		return
	}

	if hp, dp, err = checkPaths(p, id); err != nil {
		return
	}

	// hid = fmt.Sprintf("%s-H", id)
	// did = fmt.Sprintf("%s-D", id)
	hid = path.Base(hp)
	did = path.Base(dp)

	// Check the data file validty
	df, err := os.OpenFile(dp, syscall.O_RDWR, 0640)
	if err != nil {
		return
	}

	dfLock := syscall.Flock_t{
		Type:   syscall.F_WRLCK,
		Start:  0,
		Len:    0,
		Whence: 0,
	}
	df.Seek(0, 0)
	if err = syscall.FcntlFlock(df.Fd(), syscall.F_SETLK, &dfLock); err != nil {
		return
	}

	dr := bufio.NewReader(df)
	var dID []byte
	if _, err = fmt.Fscanln(dr, &dID); err != nil {
		return
	}

	if !bytes.Equal(dID, []byte(did)) {
		err = fmt.Errorf("Format error in spool file: %s", dp)
		return
	}

	// Open the header file
	hf, err := os.OpenFile(hp, syscall.O_RDWR, 0640)
	if err != nil {
		return
	}

	hfLock := syscall.Flock_t{
		Type:   syscall.F_WRLCK,
		Start:  0,
		Len:    0,
		Whence: 0,
	}
	hf.Seek(0, 0)
	if err = syscall.FcntlFlock(hf.Fd(), syscall.F_SETLK, &hfLock); err != nil {
		return
	}
	r := bufio.NewReader(hf)

	spoolErr := fmt.Errorf("Format error in spool file: %s", hp)
	tm := Msg{
		ID:       []byte(id),
		HdrFile:  hp,
		DtaFile:  dp,
		hf:       hf,
		df:       df,
		ACL:      make(map[string][]byte),
		Aclc:     make(map[string][]byte),
		Aclm:     make(map[string][]byte),
		DashVars: make(map[string][]byte),
	}
	tm.mx.Lock()
	defer tm.mx.Unlock()

	// Get the filename
	var tmphid []byte
	if _, err = fmt.Fscanln(r, &tmphid); err != nil {
		return
	}
	if !bytes.Equal(tmphid, []byte(hid)) {
		err = spoolErr
		return
	}

	// Get login, uid, and gid
	if _, err = fmt.Fscanln(r, &tm.User, &tm.UID, &tm.GID); err != nil {
		return
	}

	// Get the sender
	if _, err = fmt.Fscanln(r, &tm.Sender); err != nil {
		return
	}

	tm.Sender = bytes.Trim(tm.Sender, "\n><")
	// Get the time the message was received and number of warning
	// messages for delivery delays that have been sent.
	if _, err = fmt.Fscanln(r, &tm.Received, &tm.WarnCount); err != nil {
		return
	}

	// Get the dash variables
	var line []byte
	for {
		if line, err = r.ReadBytes('\n'); err != nil {
			return
		}

		if !bytes.HasPrefix(line, []byte("-")) {
			break
		}
		// Process acl
		if bytes.HasPrefix(line, []byte("-acl")) {
			var rem []byte
			var chars, n int
			var name, fs string
			var mapref map[string][]byte
			if bytes.HasPrefix(line, []byte("-aclc")) {
				mapref = tm.Aclc
				fs = "-aclc %s %d\n"
			} else if bytes.HasPrefix(line, []byte("-aclm")) {
				mapref = tm.Aclm
				fs = "-aclm %s %d\n"
			} else {
				mapref = tm.ACL
				fs = "-acl %s %d\n"
			}
			if _, err = fmt.Sscanf(string(line), fs, &name, &chars); err != nil {
				return
			}
			val := make([]byte, chars)
			if n, err = r.Read(val); err != nil || n != chars {
				if err != nil {
					return
				}
				err = spoolErr
				return
			}
			mapref[name] = trim(val)
			if rem, err = r.ReadBytes('\n'); err != nil || len(rem) != 1 {
				if err != nil {
					return
				}
				err = spoolErr
				return
			}
			continue
		}

		// Process no acl dash vars
		var key string
		if _, err = fmt.Sscanf(string(line), "%s ", &key); err != nil {
			return
		}
		tm.DashVars[key] = trim(line)
	}

	if len(line) == 0 {
		err = spoolErr
		return
	}

	// Get non recipients
	if !bytes.Equal(line, []byte("XX\n")) {
		tm.NonRcpts = [][]byte{trim(line)}
		for {
			if line, err = r.ReadBytes('\n'); err != nil {
				return
			}
			if bytes.HasPrefix(line, []byte("N")) || bytes.HasPrefix(line, []byte("Y")) {
				tm.NonRcpts = append(tm.NonRcpts, trim(line))
			}
			break
		}
	} else {
		tm.NonRcpts = make([][]byte, 1)
	}

	// Get num of recipients
	if _, err = fmt.Sscanf(string(line), "%d", &tm.NumRcpts); err != nil {
		return
	}

	// Get the recipients
	if line, err = r.ReadBytes('\n'); err != nil || len(line) <= 1 {
		if err == nil {
			err = spoolErr
		}
		return
	}
	tm.Rcpts = [][]byte{trim(line)}
	for i := 0; i < tm.NumRcpts-1; i++ {
		if line, err = r.ReadBytes('\n'); err != nil {
			return
		}
		tm.Rcpts = append(tm.Rcpts, trim(line))
	}

	// Get the newline seperating the headers
	if line, err = r.ReadBytes('\n'); err != nil {
		return
	}

	if string(line) != "\n" {
		err = spoolErr
		return
	}

	// Get the headers
	var flag rune
	var created bool
	var length, gsize, rsize int
	for {
		if line, err = r.ReadBytes('\n'); err != nil {
			if err == io.EOF {
				err = nil
				break
			}
			return
		}
		if _, err = fmt.Sscanf(string(line), "%d%c ", &length, &flag); err != nil {
			return
		}
		if string(flag) == "*" {
			continue
		}

		rsize = (length - len(line)) + 5
		if rsize > 0 {
			remaining := make([]byte, rsize)
			if gsize, err = r.Read(remaining); err != nil || gsize != rsize {
				if err != nil {
					return
				}
				err = spoolErr
				return
			}
			line = append(line, remaining...)
		}

		if !created {
			tm.Hdrs = [][]byte{trim(line[5:])}
			tm.RawHdrs = [][]byte{trim(line)}
			created = true
		} else {
			tm.Hdrs = append(tm.Hdrs, trim(line[5:]))
			tm.RawHdrs = append(tm.RawHdrs, trim(line))
		}
	}

	m = &tm
	return
}

func checkPaths(p, id string) (hdrPath, dataPath string, err error) {
	var stat os.FileInfo

	if stat, err = os.Stat(p); os.IsNotExist(err) {
		return
	}

	if !stat.IsDir() {
		err = fmt.Errorf(pathErr, p)
		return
	}

	hdrPath = path.Join(p, fmt.Sprintf("%s-H", id))
	if err = checkFile(hdrPath); err != nil {
		return
	}

	dataPath = path.Join(p, fmt.Sprintf("%s-D", id))
	if err = checkFile(hdrPath); err != nil {
		return
	}

	return
}

func checkFile(p string) (err error) {
	var mode os.FileMode
	var stat os.FileInfo

	if stat, err = os.Stat(p); os.IsNotExist(err) {
		return
	}

	mode = stat.Mode()

	if !mode.IsRegular() {
		err = fmt.Errorf(fileErr, p)
		return
	}

	return
}
