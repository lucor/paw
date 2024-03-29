// Copyright 2023 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Copyright 2020 Google LLC
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd

// Code in this file has been adapted from https://github.com/FiloSottile/yubikey-agent/blob/v0.1.6/main.go#L77
// released under the above license
package agent

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"golang.org/x/term"
)

func Run(a *Agent, socketPath string) {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		log.Println("Warning: paw-agent is meant to run as a background daemon.")
		log.Println("Running multiple instances is likely to lead to conflicts.")
		log.Println("Consider using the launchd or systemd services.")
	}
	c := make(chan os.Signal, 1)

	signal.Notify(c, syscall.SIGHUP)
	go func() {
		for range c {
			a.Close()
		}
	}()

	if runtime.GOOS != "windows" {
		os.Remove(socketPath)
		if err := os.MkdirAll(filepath.Dir(socketPath), 0777); err != nil {
			log.Fatalln("Failed to create UNIX socket folder:", err)
		}
	}

	l, err := listen(socketPath)
	if err != nil {
		log.Fatalln("Failed to listen on socket:", err)
	}

	for {
		c, err := l.Accept()
		if err != nil {
			type temporary interface {
				Temporary() bool
			}
			if err, ok := err.(temporary); ok && err.Temporary() {
				log.Println("Temporary Accept error, sleeping 1s:", err)
				time.Sleep(1 * time.Second)
				continue
			}
			log.Fatalln("Failed to accept connections:", err)
		}
		go a.serveConn(c)
	}
}
