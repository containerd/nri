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
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	nrinet "github.com/containerd/nri/pkg/net"
	mux "github.com/containerd/nri/pkg/net/multiplex"
)

func TestOpen(t *testing.T) {
	setup := func(t *testing.T) (mux.Mux, mux.Mux) {
		t.Helper()

		lMux, pMux, err := connectMuxes()
		require.NoError(t, err)
		require.NotNil(t, lMux)
		require.NotNil(t, pMux)
		t.Cleanup(func() {
			lMux.Close()
			pMux.Close()
		})
		return lMux, pMux
	}

	t.Run("Open should return a net.Conn", func(t *testing.T) {
		lMux, _ := setup(t)

		lConn, err := lMux.Open(mux.LowestConnID)
		require.NoError(t, err)
		require.NotNil(t, lConn)
	})

	t.Run("Opened net.Conn should allow sending", func(t *testing.T) {
		lMux, _ := setup(t)

		lConn, err := lMux.Open(mux.LowestConnID)
		require.NoError(t, err)
		require.NotNil(t, lConn)

		_, err = lConn.Write([]byte("this is a test message"))
		require.NoError(t, err)
	})

	t.Run("Opened net.Conn should allow receiving", func(t *testing.T) {
		lMux, pMux := setup(t)

		pConn, err := pMux.Open(mux.LowestConnID)
		require.NoError(t, err)
		require.NotNil(t, pConn)

		lConn, err := lMux.Open(mux.LowestConnID)
		require.NoError(t, err)
		require.NotNil(t, lConn)

		msg := "this is a test message"
		_, err = lConn.Write([]byte(msg))
		require.NoError(t, err)

		buf := make([]byte, len(msg))
		_, err = pConn.Read(buf)
		require.NoError(t, err)
		require.Equal(t, msg, string(buf))
	})
}

func TestClose(t *testing.T) {
	setup := func(t *testing.T) (net.Conn, net.Conn) {
		t.Helper()

		lMux, pMux, err := connectMuxes()
		require.NoError(t, err)
		require.NotNil(t, lMux)
		require.NotNil(t, pMux)
		t.Cleanup(func() {
			lMux.Close()
			pMux.Close()
		})

		lConn, err := lMux.Open(mux.LowestConnID)
		require.NoError(t, err)
		require.NotNil(t, lConn)
		pConn, err := pMux.Open(mux.LowestConnID)
		require.NoError(t, err)
		require.NotNil(t, pConn)
		return lConn, pConn
	}

	t.Run("Closed connection should fail sending", func(t *testing.T) {
		lConn, _ := setup(t)

		msg := "this is a test message"
		_, err := lConn.Write([]byte(msg))
		require.NoError(t, err)

		require.NoError(t, lConn.Close())

		_, err = lConn.Write([]byte(msg))
		require.Error(t, err)
	})

	t.Run("Closed connection should fail receiving", func(t *testing.T) {
		_, pConn := setup(t)

		require.NoError(t, pConn.Close())

		buf := make([]byte, 64)
		_, err := pConn.Read(buf)
		require.Error(t, err)
	})
}

func TestDial(t *testing.T) {
	setup := func(t *testing.T) mux.Mux {
		t.Helper()

		lMux, pMux, err := connectMuxes()
		require.NoError(t, err)
		require.NotNil(t, lMux)
		require.NotNil(t, pMux)
		t.Cleanup(func() {
			lMux.Close()
			pMux.Close()
		})
		return lMux
	}

	dial := func(m mux.Mux, connID mux.ConnID) (net.Conn, error) {
		return m.Dialer(connID)("mux", "id")
	}

	t.Run("Dial should return a net.Conn", func(t *testing.T) {
		lMux := setup(t)

		conn, err := dial(lMux, mux.LowestConnID)
		require.NoError(t, err)
		require.NotNil(t, conn)
	})

	t.Run("Dialed net.Conn should allow sending", func(t *testing.T) {
		lMux := setup(t)

		conn, err := dial(lMux, mux.LowestConnID)
		require.NoError(t, err)
		require.NotNil(t, conn)

		_, err = conn.Write([]byte("this is a test message"))
		require.NoError(t, err)
	})
}

func TestListenAccept(t *testing.T) {
	setup := func(t *testing.T) (mux.Mux, mux.Mux) {
		t.Helper()

		lMux, pMux, err := connectMuxes()
		require.NoError(t, err)
		require.NotNil(t, lMux)
		require.NotNil(t, pMux)
		t.Cleanup(func() {
			lMux.Close()
			pMux.Close()
		})
		return lMux, pMux
	}

	accept := func(m mux.Mux, connID mux.ConnID) (net.Listener, net.Conn, error) {
		l, err := m.Listen(connID)
		if err != nil {
			return nil, nil, err
		}
		conn, err := l.Accept()
		return l, conn, err
	}

	t.Run("Listen should return a net.Listener", func(t *testing.T) {
		_, pMux := setup(t)

		l, err := pMux.Listen(mux.LowestConnID)
		require.NoError(t, err)
		require.NotNil(t, l)
	})

	t.Run("Accept on the net.Listener should return a net.Conn", func(t *testing.T) {
		_, pMux := setup(t)

		_, pConn, err := accept(pMux, mux.LowestConnID)
		require.NoError(t, err)
		require.NotNil(t, pConn)
	})

	t.Run("Accepted net.Conn should allow receiving", func(t *testing.T) {
		lMux, pMux := setup(t)

		_, pConn, err := accept(pMux, mux.LowestConnID)
		require.NoError(t, err)
		require.NotNil(t, pConn)

		lConn, err := lMux.Open(mux.LowestConnID)
		require.NoError(t, err)
		require.NotNil(t, lConn)

		msg := "this is a test message"
		_, err = lConn.Write([]byte(msg))
		require.NoError(t, err)

		buf := make([]byte, len(msg))
		_, err = pConn.Read(buf)
		require.NoError(t, err)
		require.Equal(t, msg, string(buf))
	})
}

func TestTransmittingData(t *testing.T) {
	setup := func(t *testing.T, connCnt int) ([]net.Conn, []net.Conn) {
		t.Helper()

		lMux, pMux, err := connectMuxes()
		require.NoError(t, err)
		require.NotNil(t, lMux)
		require.NotNil(t, pMux)
		t.Cleanup(func() {
			lMux.Close()
			pMux.Close()
		})

		lConn, pConn, err := openMuxes(lMux, pMux, connCnt)
		require.NoError(t, err)
		require.Len(t, lConn, connCnt)
		require.Len(t, pConn, connCnt)
		return lConn, pConn
	}

	t.Run("single connection", func(t *testing.T) {
		lConn, pConn := setup(t, 1)
		require.NoError(t, sendAndReceive(lConn, pConn, 64))
	})

	t.Run("multiple connections", func(t *testing.T) {
		lConn, pConn := setup(t, 16)
		require.NoError(t, sendAndReceive(lConn, pConn, 64))
	})

	t.Run("an oversized message is transmitted in multiple chunks", func(t *testing.T) {
		const (
			maxPayloadSize = 10 + 4<<20
			overflowFactor = 3
		)

		lConn, pConn := setup(t, 1)

		msg := strings.Repeat("a", overflowFactor*maxPayloadSize)
		cnt, err := lConn[0].Write([]byte(msg))
		require.NoError(t, err)
		require.Equal(t, len(msg), cnt)

		rcv := make([]byte, overflowFactor*maxPayloadSize)
		size := 0
		for i := 0; size < len(msg) && i < overflowFactor; i++ {
			cnt, err := pConn[0].Read(rcv[size:])
			require.NoError(t, err)
			require.Equal(t, maxPayloadSize, cnt)
			size += cnt
		}
		require.Equal(t, []byte(msg), rcv)

		msg = strings.Repeat("b", 200)
		cnt, err = lConn[0].Write([]byte(msg))
		require.NoError(t, err)
		require.Equal(t, len(msg), cnt)

		rcv = make([]byte, len(msg))
		cnt, err = pConn[0].Read(rcv)
		require.NoError(t, err)
		require.Equal(t, len(msg), cnt)
		require.Equal(t, []byte(msg), rcv)
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

func sendAndReceive(lConn, pConn []net.Conn, msgCount int) error {
	const maxMsg = 64

	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make(chan error, 1)

	var failOnce sync.Once
	fail := func(err error) {
		failOnce.Do(func() {
			errs <- err
			for _, conn := range lConn {
				conn.Close()
			}
			for _, conn := range pConn {
				conn.Close()
			}
		})
	}

	write := func(id int, conn net.Conn, messages []string) ([]string, error) {
		if messages == nil {
			for i := 0; i < msgCount; i++ {
				msg := fmt.Sprintf("[%d] message #%d/%d", id, i+1, msgCount)
				if len(msg) > maxMsg {
					return nil, fmt.Errorf("message length %d exceeds maximum %d", len(msg), maxMsg)
				}
				messages = append(messages, msg)
			}
		}

		for _, msg := range messages {
			cnt, err := conn.Write([]byte(msg))
			if err != nil {
				return nil, err
			}
			if cnt != len(msg) {
				return nil, fmt.Errorf("wrote %d bytes, expected %d", cnt, len(msg))
			}
		}

		cnt, err := conn.Write(nil)
		if err != nil {
			return nil, err
		}
		if cnt != 0 {
			return nil, fmt.Errorf("wrote %d bytes for end message, expected 0", cnt)
		}

		return messages, nil
	}

	read := func(conn net.Conn) ([]string, error) {
		msg := make([]byte, maxMsg)
		var recv []string
		for {
			cnt, err := conn.Read(msg)
			if err != nil {
				return nil, err
			}
			if cnt == 0 {
				return recv, nil
			}
			recv = append(recv, string(msg[:cnt]))
		}
	}

	sendrecv := func(id int, conn net.Conn, sender bool) {
		defer wg.Done()
		<-start

		if sender {
			sent, err := write(id, conn, nil)
			if err != nil {
				fail(err)
				return
			}
			recv, err := read(conn)
			if err != nil {
				fail(err)
				return
			}
			if !equalStrings(sent, recv) {
				fail(errors.New("sent and received messages differ"))
			}
			return
		}

		recv, err := read(conn)
		if err != nil {
			fail(err)
			return
		}
		if _, err := write(id, conn, recv); err != nil {
			fail(err)
		}
	}

	for i := 0; i < len(lConn); i++ {
		wg.Add(2)
		go sendrecv(i, lConn[i], true)
		go sendrecv(i, pConn[i], false)
	}

	close(start)
	wg.Wait()

	select {
	case err := <-errs:
		return err
	default:
		return nil
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
