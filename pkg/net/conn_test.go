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

package net_test

import (
	"bufio"
	"errors"
	"fmt"
	"io"

	"github.com/containerd/nri/pkg/net"

	"testing"

	require "github.com/stretchr/testify/require"
)

func TestPreconnectedReadWrite(t *testing.T) {
	sp, err := net.NewSocketPair()
	require.NoError(t, err, "NewSocketPair()")
	require.NotNil(t, sp, "NewSocketPair()")

	conn, err := sp.LocalConn()
	require.NoError(t, err, "LocalConn()")
	require.NotNil(t, conn, "LocalConn()")

	sent := []string{}
	recv := []string{}
	done := make(chan error, 1)

	reader := func() {
		conn, err := sp.PeerConn()
		require.NoError(t, err, "PeerConn()")
		require.NotNil(t, conn, "PeerConn()")

		l := net.NewConnListener(conn)
		require.NotNil(t, l, "NewConnListener()")

		pconn, err := l.Accept()
		require.NoError(t, err, "Accept()")
		require.NotNil(t, pconn, "Accept()")

		buf := bufio.NewReader(pconn)
		for {
			msg, err := buf.ReadString('\n')
			if err != nil {
				if !errors.Is(err, io.EOF) {
					done <- fmt.Errorf("ReadString() failed: %w", err)
				}
				break
			}
			recv = append(recv, msg)
		}
		close(done)
	}

	go reader()

	l := net.NewConnListener(conn)
	require.NotNil(t, l, "NewConnListener()")

	lconn, err := l.Accept()
	require.NoError(t, err, "Accept()")
	require.NotNil(t, lconn, "Accept()")

	for i := 0; i < 32; i++ {
		msg := fmt.Sprintf("message #%d\n", i)
		_, err := lconn.Write([]byte(msg))
		require.NoError(t, err, "Write()")
		sent = append(sent, msg)
	}
	lconn.Close()

	err = <-done
	require.NoError(t, err, "done/reader")
	require.Equal(t, sent, recv, "send and received data")
}
