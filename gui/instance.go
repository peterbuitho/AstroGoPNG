package main

import (
	"bufio"
	"net"
	"strings"
	"time"
)

// Single-instance handoff. Windows Explorer starts one process per selected
// file for a right-click verb (and drag-and-drop onto the .exe does the same);
// the first process to bind the local port becomes the window, and every later
// one hands it its paths and exits, so a multi-selection ends up as one file
// list in one window.
//
// Plain std TCP on localhost, no dependencies. Protocol: a header line followed
// by one absolute path per line, then a one-byte ack from the primary.
//
// Port of the Rust src/bin/gui/instance.rs.

const (
	instAddr   = "127.0.0.1:47820"
	instHeader = "astrogopng-open 1"
)

// claim tries to become the primary instance. On success it returns the
// listener to keep (the caller starts serve once the UI exists). It returns
// (nil, false) when another instance already runs and `paths` were handed to
// it — the caller should exit.
func claim(paths []string) (net.Listener, bool) {
	ln, err := net.Listen("tcp", instAddr)
	if err == nil {
		return ln, true
	}
	if forward(paths) {
		return nil, false
	}
	// Port taken by something that is not us: run standalone on a random port.
	ln, _ = net.Listen("tcp", "127.0.0.1:0")
	return ln, true
}

func forward(paths []string) bool {
	for attempt := 0; attempt < 5; attempt++ {
		if attempt > 0 {
			time.Sleep(150 * time.Millisecond)
		}
		conn, err := net.DialTimeout("tcp", instAddr, 500*time.Millisecond)
		if err != nil {
			continue
		}
		_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		var b strings.Builder
		b.WriteString(instHeader)
		b.WriteByte('\n')
		for _, p := range paths {
			b.WriteString(p)
			b.WriteByte('\n')
		}
		if _, err := conn.Write([]byte(b.String())); err != nil {
			_ = conn.Close()
			continue
		}
		if tc, ok := conn.(*net.TCPConn); ok {
			_ = tc.CloseWrite()
		}
		// Wait for the primary's ack so we don't exit before it has read.
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		ack := make([]byte, 1)
		_, _ = conn.Read(ack)
		_ = conn.Close()
		return true
	}
	return false
}

// serve accepts handoffs forever, calling onPaths with each received batch.
func serve(ln net.Listener, onPaths func([]string)) {
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
				r := bufio.NewReader(c)
				header, err := r.ReadString('\n')
				if err != nil || strings.TrimRight(header, "\r\n") != instHeader {
					return
				}
				var paths []string
				for {
					line, err := r.ReadString('\n')
					if p := strings.TrimSpace(line); p != "" {
						paths = append(paths, p)
					}
					if err != nil {
						break
					}
				}
				_, _ = c.Write([]byte("k"))
				if len(paths) > 0 {
					onPaths(paths)
				}
			}(conn)
		}
	}()
}
