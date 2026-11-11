// SPDX-License-Identifier: MIT

package netproxy

import (
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var bufPool = sync.Pool{New: func() any { b := make([]byte, 32*1024); return &b }}

// Pipe relays bytes bidirectionally between client and upstream until either side
// closes or neither direction transfers data for idleTimeout.
// onSent is invoked for bytes written from client -> upstream.
// onRecv is invoked for bytes written from upstream -> client.
func Pipe(client, upstream net.Conn, onSent, onRecv func(int64)) (sentTotal, recvTotal int64) {
	var last atomic.Int64
	last.Store(time.Now().UnixNano())

	var sentAtom, recvAtom atomic.Int64
	done := make(chan struct{}, 2)

	cp := func(dst, src net.Conn, counter *atomic.Int64, cb func(int64)) {
		bp := bufPool.Get().(*[]byte)
		defer bufPool.Put(bp)
		buf := *bp
		for {
			_ = src.SetReadDeadline(time.Now().Add(idleTimeout))
			n, err := src.Read(buf)
			if n > 0 {
				last.Store(time.Now().UnixNano())
				written, werr := dst.Write(buf[:n])
				if written > 0 {
					counter.Add(int64(written))
					if cb != nil {
						cb(int64(written))
					}
				}
				if werr != nil {
					break
				}
			}
			if ne, ok := err.(net.Error); ok && ne.Timeout() &&
				time.Since(time.Unix(0, last.Load())) < idleTimeout {
				continue
			}
			if err != nil {
				break
			}
		}
		if tc, ok := dst.(interface{ CloseWrite() error }); ok {
			_ = tc.CloseWrite()
		} else {
			_ = dst.Close()
		}
		done <- struct{}{}
	}

	go cp(upstream, client, &sentAtom, onSent)
	go cp(client, upstream, &recvAtom, onRecv)

	<-done
	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}
	_ = client.Close()
	_ = upstream.Close()

	return sentAtom.Load(), recvAtom.Load()
}
