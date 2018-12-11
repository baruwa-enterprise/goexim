// Copyright (C) 2018 Andrew Colin Kissa <andrew@datopdog.io>
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at http://mozilla.org/MPL/2.0/.

/*
Package spoolfile Reads and parses Exim spool files
*/
package spoolfile

import (
	"bytes"
	"fmt"
	"go/build"
	"os"
	"path"
	"strings"
	"testing"
)

var (
	gopath string
)

func init() {
	gopath = os.Getenv("GOPATH")
	if gopath == "" {
		gopath = build.Default.GOPATH
	}
}

func TestBasics(t *testing.T) {
	var p, id, hf, df, did, hid, user, sender, heloname string
	var uid, gid, rcvd, wc int
	var err error
	var msg *Msg
	uid = 93
	gid = 93
	rcvd = 1515239630
	user = "exim"
	heloname = "-helo_name alcazar.home.topdog-software.com"
	sender = "andrew@kudusoft.home.topdog-software.com"
	id = "1eXn2s-0008DG-EX"
	p = path.Join(gopath, "src/github.com/baruwa-enterprise/goexim/testdata")
	hid = fmt.Sprintf("%s-H", id)
	did = fmt.Sprintf("%s-D", id)
	hf = path.Join(p, hid)
	df = path.Join(p, did)
	if msg, err = NewMsg(p, id); err != nil {
		t.Fatalf("UnExpected error: %s", err)
	}
	defer msg.Close()

	if !bytes.Equal(msg.ID, []byte(id)) {
		t.Errorf("Got %q want %q", msg.ID, id)
	}

	if !bytes.Equal(msg.User, []byte(user)) {
		t.Errorf("Got %q want %q", msg.User, user)
	}

	if msg.UID != uid {
		t.Errorf("Got %d want %d", msg.UID, uid)
	}

	if msg.GID != gid {
		t.Errorf("Got %d want %d", msg.GID, gid)
	}

	if !bytes.Equal(msg.Sender, []byte(sender)) {
		t.Errorf("Got %q want %q", msg.Sender, sender)
	}

	if msg.Received != rcvd {
		t.Errorf("Got %d want %d", msg.Received, rcvd)
	}

	if msg.WarnCount != wc {
		t.Errorf("Got %d want %d", msg.WarnCount, wc)
	}

	if !bytes.Equal(msg.DashVars["-helo_name"], []byte(heloname)) {
		t.Errorf("Got %q want %q", msg.DashVars["-helo_name"], heloname)
	}

	if msg.HdrFile != hf {
		t.Errorf("Got %q want %q", msg.HdrFile, hf)
	}

	if msg.DtaFile != df {
		t.Errorf("Got %q want %q", msg.DtaFile, df)
	}
}

func TestBody(t *testing.T) {
	var err error
	var msg *Msg
	var p, id, tb, mb string
	var body []byte
	id = "1eXn2s-0008DG-EX"
	tb = "This is a test mailing\n\n"
	p = path.Join(gopath, "src/github.com/baruwa-enterprise/goexim/testdata")
	if msg, err = NewMsg(p, id); err != nil {
		t.Fatalf("UnExpected error: %s", err)
	}
	defer msg.Close()

	if body, err = msg.Body(); err != nil {
		t.Fatalf("UnExpected error: %s", err)
	}
	if !bytes.Equal(body, []byte(tb)) {
		t.Errorf("Got %q want %q", body, tb)
	}

	if mb, err = msg.String(); err != nil {
		t.Fatalf("UnExpected error: %s", err)
	}
	if !strings.HasSuffix(mb, tb) {
		t.Errorf("Got %q want %q", mb, tb)
	}
}

func TestErrors(t *testing.T) {
	var p, id, emsg string
	var err error
	p = path.Join(gopath, "src/github.com/baruwa-enterprise/goexim/testdata/1eXn2s-0008DG-EX-H")
	id = "1eXn2s-0008DG-EX"
	emsg = fmt.Sprintf(pathErr, p)
	if _, err = NewMsg(p, id); err == nil {
		t.Fatalf("An error should be returned")
	}

	if err.Error() != emsg {
		t.Errorf("Got %q want %q", err, emsg)
	}

	id = "1cGaid-0000sq-9g"
	p = path.Join(gopath, "src/github.com/baruwa-enterprise/goexim/testdata")
	emsg = fmt.Sprintf("stat %s/%s-H: no such file or directory", p, id)
	if _, err = NewMsg(p, id); err == nil {
		t.Fatalf("An error should be returned")
	}

	if err.Error() != emsg {
		t.Errorf("Got %q want %q", err, emsg)
	}
}
