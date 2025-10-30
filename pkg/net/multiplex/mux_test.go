/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package multiplex_test

import (
	"fmt"
	"net"
	"strings"
	"sync"

	nrinet "github.com/containerd/nri/pkg/net"
	mux "github.com/containerd/nri/pkg/net/multiplex"

	"testing"

	"github.com/stretchr/testify/require"
)

func TestMux(t *testing.T) {
	var (
		lMux, pMux   mux.Mux
		connID       mux.ConnID
		lConn, pConn net.Conn
		l            net.Listener
		err          error
		n            int

		setup = func(t *testing.T) {
			lMux, pMux, err = connectMuxes()
			require.NoError(t, err, "multiplexed socketpair")
			require.NotNil(t, lMux, "non-nil mux")
			require.NotNil(t, pMux, "non-nil mux")
			connID = mux.LowestConnID
		}
		cleanup = func() {
			if lMux != nil {
				lMux.Close()
			}
			if pMux != nil {
				pMux.Close()
			}
		}
		dial = func(m mux.Mux, connID mux.ConnID) (net.Conn, error) {
			return m.Dialer(connID)("mux", "id")
		}

		accept = func(m mux.Mux, connID mux.ConnID) (net.Conn, error) {
			l, err = m.Listen(connID)
			if err != nil {
				return nil, err
			}
			return l.Accept()
		}
	)

	t.Run("Open should return a net.Conn", func(t *testing.T) {
		setup(t)

		lConn, err = lMux.Open(connID)
		require.NoError(t, err, "open connection without error")
		require.NotNil(t, lConn, "opened connection not nil")

		cleanup()
	})

	t.Run("Opened net.Conn should allow sending", func(t *testing.T) {
		setup(t)

		lConn, err = lMux.Open(connID)
		require.NoError(t, err, "open connection without error")
		require.NotNil(t, lConn, "opened connection not nil")

		_, err = lConn.Write([]byte("this is a test message"))
		require.NoError(t, err, "write to opened connection without error")

		cleanup()
	})

	t.Run("Opened net.Conn should allow receiving", func(t *testing.T) {
		setup(t)

		pConn, err = pMux.Open(connID)
		require.NoError(t, err, "open connection without error")
		require.NotNil(t, pConn, "opened connection not nil")

		lConn, err = lMux.Open(connID)
		require.NoError(t, err, "open connection without error")
		require.NotNil(t, lConn, "opened connection not nil")

		msg := "this is a test message"
		n, err = lConn.Write([]byte(msg))
		require.NoError(t, err, "write to opened connection without error")
		require.Equal(t, len(msg), n, "written bytes matches message length")

		buf := make([]byte, len(msg))
		n, err := pConn.Read(buf)
		require.NoError(t, err, "read from opened connection without error")
		require.Equal(t, msg, string(buf[:n]), "received message matches sent message")

		cleanup()
	})

	t.Run("Closed connection should fail sending", func(t *testing.T) {
		setup(t)

		lConn, err = lMux.Open(connID)
		require.NoError(t, err, "open connection without error")
		require.NotNil(t, lConn, "opened connection not nil")

		pConn, err = pMux.Open(connID)
		require.NoError(t, err, "open connection without error")
		require.NotNil(t, pConn, "opened connection not nil")

		msg := "this is a test message"
		n, err = lConn.Write([]byte(msg))
		require.NoError(t, err, "write to opened connection without error")
		require.Equal(t, len(msg), n, "written bytes matches message length")

		err = lConn.Close()
		require.NoError(t, err, "close opened connection without error")

		_, err = lConn.Write([]byte(msg))
		require.Error(t, err, "write to closed connection returns error")

		cleanup()
	})

	t.Run("Closed connection should fail receiving", func(t *testing.T) {
		setup(t)

		lConn, err = lMux.Open(connID)
		require.NoError(t, err, "open connection without error")
		require.NotNil(t, lConn, "opened connection not nil")

		pConn, err = pMux.Open(connID)
		require.NoError(t, err, "open connection without error")
		require.NotNil(t, pConn, "opened connection not nil")

		err = pConn.Close()
		require.NoError(t, err, "close opened connection without error")

		buf := make([]byte, 64)
		_, err = pConn.Read(buf)
		require.Error(t, err, "read from closed connection returns error")

		cleanup()
	})

	t.Run("dial should return a net.Conn", func(t *testing.T) {
		setup(t)

		lConn, err = dial(lMux, connID)
		require.NoError(t, err, "dial connection without error")
		require.NotNil(t, lConn, "dialed connection not nil")

		cleanup()
	})

	t.Run("Dialed net.Conn should allow sending", func(t *testing.T) {
		setup(t)

		lConn, err = dial(lMux, connID)
		require.NoError(t, err, "dial connection without error")
		require.NotNil(t, lConn, "dialed connection not nil")

		_, err = lConn.Write([]byte("this is a test message"))
		require.NoError(t, err, "write to dialed connection without error")

		cleanup()
	})

	t.Run("Listen should return a net.Listener", func(t *testing.T) {
		setup(t)

		l, err = pMux.Listen(connID)
		require.NoError(t, err, "listen without error")
		require.NotNil(t, l, "listener not nil")

		cleanup()
	})

	t.Run("Accept on the listener should return a net.Conn", func(t *testing.T) {
		setup(t)

		pConn, err = accept(pMux, connID)
		require.NoError(t, err, "accept connection without error")
		require.NotNil(t, pConn, "accepted connection not nil")

		cleanup()
	})

	t.Run("Accepted net.Conn should allow receiving", func(t *testing.T) {
		setup(t)

		pConn, err = accept(pMux, connID)
		require.NoError(t, err, "accept connection without error")
		require.NotNil(t, pConn, "accepted connection not nil")

		lConn, err = lMux.Open(connID)
		require.NoError(t, err, "open connection without error")
		require.NotNil(t, lConn, "opened connection not nil")

		msg := "this is a test message"
		_, err = lConn.Write([]byte(msg))
		require.NoError(t, err, "write to opened connection without error")

		buf := make([]byte, len(msg))
		n, err := pConn.Read(buf)
		require.NoError(t, err, "read from accepted connection without error")
		require.Equal(t, msg, string(buf[:n]), "received message matches sent message")

		cleanup()
	})

	t.Run("transmitting data over a single connection", func(t *testing.T) {
		var (
			connCnt = 1
			msgCnt  = 64
		)

		setup(t)

		lc, pc, err := openMuxes(lMux, pMux, connCnt)
		require.NoError(t, err, "open a single connection without error")
		require.NotNil(t, lc, "opened connection not nil")
		require.NotNil(t, pc, "opened connection not nil")
		require.Equal(t, connCnt, len(lc), "opened connection count matches")

		sendAndReceive(t, lc, pc, msgCnt)

		cleanup()
	})

	t.Run("transmitting data over multiple connections", func(t *testing.T) {
		var (
			connCnt = 16
			msgCnt  = 64
		)

		setup(t)

		lc, pc, err := openMuxes(lMux, pMux, connCnt)
		require.NoError(t, err, "open multiple connections without error")
		require.NotNil(t, lc, "opened connections not nil")
		require.NotNil(t, pc, "opened connections not nil")
		require.Equal(t, connCnt, len(lc), "opened connection count matches")

		sendAndReceive(t, lc, pc, msgCnt)

		cleanup()
	})

	t.Run("oversized messages are transmitted (in multiple chunks)", func(t *testing.T) {
		var (
			maxPayloadSize = 10 + 4<<20
			overflowFactor = 3
		)

		setup(t)

		lConn, err = lMux.Open(connID)
		require.NoError(t, err, "open connection without error")
		require.NotNil(t, lConn, "opened connections not nil")

		pConn, err = pMux.Open(connID)
		require.NoError(t, err, "open connection without error")
		require.NotNil(t, pConn, "opened connections not nil")

		msg := strings.Repeat("a", overflowFactor*maxPayloadSize)
		cnt, err := lConn.Write([]byte(msg))
		require.NoError(t, err, "write oversized message without error")
		require.Equal(t, len(msg), cnt, "written bytes matches message length")

		rcv := make([]byte, overflowFactor*maxPayloadSize)
		size := 0
		for i := 0; size < len([]byte(msg)) && i < overflowFactor; i++ {
			cnt, err := pConn.Read(rcv[size:])
			require.NoError(t, err, "read chunk of oversized message without error")
			require.Equal(t, maxPayloadSize, cnt)
			size += cnt
		}
		require.Equal(t, []byte(msg), rcv, "received message matches sent one")

		cleanup()
	})

}

// getSocketPairConn returns connections for a socketpair.
func getSocketPairConn() (net.Conn, net.Conn, error) {
	fds, err := nrinet.NewSocketPair()
	if err != nil {
		return nil, nil, err
	}

	lConn, err := fds.LocalConn()
	if err != nil {
		fds.LocalClose()
		fds.PeerClose()
		return nil, nil, err
	}
	pConn, err := fds.PeerConn()
	if err != nil {
		fds.LocalClose()
		fds.PeerClose()
		return nil, nil, err
	}

	return lConn, pConn, nil
}

// connectMuxes returns a pair of connected muxes.
func connectMuxes(options ...mux.Option) (mux.Mux, mux.Mux, error) {
	lConn, pConn, err := getSocketPairConn()
	if err != nil {
		return nil, nil, err
	}
	return mux.Multiplex(lConn, options...), mux.Multiplex(pConn, options...), nil
}

// openMuxes opens a number of connections for a pair of connected muxes.
func openMuxes(lMux, pMux mux.Mux, count int) ([]net.Conn, []net.Conn, error) {
	var (
		lConn []net.Conn
		pConn []net.Conn
		conn  net.Conn
		err   error
	)

	for i := 0; i < count; i++ {
		conn, err = lMux.Open(mux.LowestConnID + mux.ConnID(i))
		if err != nil {
			lMux.Trunk().Close()
			pMux.Trunk().Close()
			return nil, nil, err
		}
		lConn = append(lConn, conn)

		conn, err = pMux.Open(mux.LowestConnID + mux.ConnID(i))
		if err != nil {
			lMux.Trunk().Close()
			pMux.Trunk().Close()
			return nil, nil, err
		}
		pConn = append(pConn, conn)
	}

	return lConn, pConn, nil
}

// Send and receive data over a set of connections.
func sendAndReceive(t *testing.T, lConn, pConn []net.Conn, msgCount int) {
	var (
		wg     = &sync.WaitGroup{}
		start  = make(chan struct{})
		maxMsg = 64
		endMsg = ""
	)

	// message sender
	write := func(id int, conn net.Conn, messages []string) []string {
		var (
			msg string
			cnt int
			err error
		)

		if messages == nil {
			for i := 0; i < msgCount; i++ {
				msg := fmt.Sprintf("[%d] message #%d/%d", id, i+1, msgCount)
				require.True(t, len(msg) <= maxMsg, "expected message length")
				messages = append(messages, msg)
			}
		}

		for _, msg = range messages {
			cnt, err = conn.Write([]byte(msg))
			require.NoError(t, err, "write message without error")
			require.Equal(t, len(msg), cnt, "written bytes matches message length")
		}

		cnt, err = conn.Write([]byte(endMsg))
		require.NoError(t, err, "write end message without error")
		require.Equal(t, len(endMsg), cnt, "written bytes matches end message length")

		return messages
	}

	// message receiver and collector
	read := func(conn net.Conn) []string {
		var (
			msg  = make([]byte, maxMsg)
			recv []string
			cnt  int
			err  error
		)

		for {
			cnt, err = conn.Read(msg)
			require.NoError(t, err, "read message without error")
			if cnt == 0 {
				return recv
			}
			recv = append(recv, string(msg[:cnt]))
		}
	}

	// send and receive, or the other way around, check echoed messages for equality
	sendrecv := func(id int, conn net.Conn, sender bool) {
		var (
			sent []string
			recv []string
		)

		defer wg.Done()
		<-start

		if sender {
			sent = write(id, conn, nil)
			recv = read(conn)
			require.Equal(t, sent, recv, "sent and received messages match")
		} else {
			recv = read(conn)
			write(id, conn, recv)
		}
	}

	// set up senders and receivers, waiting for a trigger to start
	for i := 0; i < len(lConn); i++ {
		go sendrecv(i, lConn[i], true)
		go sendrecv(i, pConn[i], false)
		wg.Add(2)
	}

	// trigger senders/receivers and wait for them to finish
	close(start)
	wg.Wait()
}
