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
	"sync"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	nrinet "github.com/containerd/nri/pkg/net"
	mux "github.com/containerd/nri/pkg/net/multiplex"
)

func TestMultiplex(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Connection Multiplexer")
}

var _ = Describe("Emulated Connection Setup, Open", func() {
	var (
		lMux, pMux   mux.Mux
		connID       mux.ConnID
		lConn, pConn net.Conn
		err          error
	)

	BeforeEach(func() {
		lMux, pMux, err = connectMuxes()
		Expect(err).To(BeNil())
		Expect(lMux).ToNot(BeNil())
		Expect(pMux).ToNot(BeNil())
		connID = mux.LowestConnID
	})

	AfterEach(func() {
		if lMux != nil {
			lMux.Close()
		}
		if pMux != nil {
			pMux.Close()
		}
	})

	It("Open should return a net.Conn", func() {
		// When
		lConn, err = lMux.Open(connID)

		// Then
		Expect(err).To(BeNil())
		Expect(lConn).ToNot(BeNil())
	})

	It("Opened net.Conn should allow sending", func() {
		// Given
		lConn, err = lMux.Open(connID)
		Expect(err).To(BeNil())
		Expect(lConn).ToNot(BeNil())

		// When
		_, err = lConn.Write([]byte("this is a test message"))

		// Then
		Expect(err).To(BeNil())
	})

	It("Opened net.Conn should allow receiving", func() {
		// Given
		pConn, err = pMux.Open(connID)
		Expect(err).To(BeNil())
		Expect(pConn).ToNot(BeNil())

		// When
		lConn, err = lMux.Open(connID)
		Expect(err).To(BeNil())
		Expect(lConn).ToNot(BeNil())

		msg := "this is a test message"
		_, err = lConn.Write([]byte(msg))
		Expect(err).To(BeNil())

		// Then
		buf := make([]byte, len(msg))
		_, err = pConn.Read(buf)
		Expect(err).To(BeNil())
		Expect(string(buf)).To(Equal(msg))
	})

})

var _ = Describe("Emulated Connection Setup, Close", func() {
	var (
		lMux, pMux   mux.Mux
		connID       mux.ConnID
		lConn, pConn net.Conn
		err          error
	)

	BeforeEach(func() {
		lMux, pMux, err = connectMuxes()
		Expect(err).To(BeNil())
		Expect(lMux).ToNot(BeNil())
		Expect(pMux).ToNot(BeNil())

		connID = mux.LowestConnID
		lConn, err = lMux.Open(connID)
		Expect(err).To(BeNil())
		Expect(lConn).ToNot(BeNil())
		pConn, err = pMux.Open(connID)
		Expect(err).To(BeNil())
		Expect(pConn).ToNot(BeNil())
	})

	AfterEach(func() {
		if lMux != nil {
			lMux.Close()
		}
		if pMux != nil {
			pMux.Close()
		}
	})

	It("Closed connection should fail sending", func() {
		// Given
		msg := "this is a test message"
		_, err = lConn.Write([]byte(msg))
		Expect(err).To(BeNil())

		// When
		err = lConn.Close()
		Expect(err).To(BeNil())

		// Then
		_, err = lConn.Write([]byte(msg))
		Expect(err).ToNot(BeNil())
	})

	It("Closed connection should fail receiving", func() {
		// Given
		err = pConn.Close()
		Expect(err).To(BeNil())

		// When
		buf := make([]byte, 64)
		_, err = pConn.Read(buf)

		// Then
		Expect(err).ToNot(BeNil())
	})
})

var _ = Describe("Emulated Connection Setup, Dial", func() {
	var (
		lMux, pMux mux.Mux
		connID     mux.ConnID
		conn       net.Conn
		err        error
	)

	BeforeEach(func() {
		lMux, pMux, err = connectMuxes()
		Expect(err).To(BeNil())
		Expect(lMux).ToNot(BeNil())
		Expect(pMux).ToNot(BeNil())
		connID = mux.LowestConnID
	})

	AfterEach(func() {
		if lMux != nil {
			lMux.Close()
		}
		if pMux != nil {
			pMux.Close()
		}
	})

	dial := func(m mux.Mux, connID mux.ConnID) (net.Conn, error) {
		return m.Dialer(connID)("mux", "id")
	}

	It("Dial should return a net.Conn", func() {
		// When
		conn, err = dial(lMux, connID)

		// Then
		Expect(err).To(BeNil())
		Expect(conn).ToNot(BeNil())
	})

	It("Dialed net.Conn should allow sending", func() {
		// Given
		conn, err = dial(lMux, connID)
		Expect(err).To(BeNil())
		Expect(conn).ToNot(BeNil())

		// When
		_, err = conn.Write([]byte("this is a test message"))

		// Then
		Expect(err).To(BeNil())
	})

})

var _ = Describe("Emulated Connection Setup, Listen, Accept", func() {
	var (
		lMux, pMux   mux.Mux
		connID       mux.ConnID
		l            net.Listener
		lConn, pConn net.Conn
		err          error
	)

	BeforeEach(func() {
		lMux, pMux, err = connectMuxes()
		Expect(err).To(BeNil())
		Expect(lMux).ToNot(BeNil())
		Expect(pMux).ToNot(BeNil())
		connID = mux.LowestConnID
	})

	AfterEach(func() {
		if lMux != nil {
			lMux.Close()
		}
		if pMux != nil {
			pMux.Close()
		}
	})

	accept := func(m mux.Mux, connID mux.ConnID) (net.Conn, error) {
		l, err = m.Listen(connID)
		if err != nil {
			return nil, err
		}
		return l.Accept()
	}

	It("Listen should return a net.Listener", func() {
		// When
		l, err = pMux.Listen(connID)

		// Then
		Expect(err).To(BeNil())
		Expect(l).ToNot(BeNil())
	})

	It("Accept on the net.Listener should return a net.Conn", func() {
		// When
		pConn, err = accept(pMux, connID)

		// Then
		Expect(err).To(BeNil())
		Expect(pConn).ToNot(BeNil())
	})

	It("Accepted net.Conn should allow receiving", func() {
		// Given
		pConn, err = accept(pMux, connID)
		Expect(err).To(BeNil())
		Expect(pConn).ToNot(BeNil())

		// When
		lConn, err = lMux.Open(connID)
		Expect(err).To(BeNil())
		Expect(lConn).ToNot(BeNil())

		msg := "this is a test message"
		_, err = lConn.Write([]byte(msg))
		Expect(err).To(BeNil())

		// Then
		buf := make([]byte, len(msg))
		_, err = pConn.Read(buf)
		Expect(err).To(BeNil())
		Expect(string(buf)).To(Equal(msg))
	})

})

var _ = Describe("Transmitting data", func() {
	var (
		lMux mux.Mux
		pMux mux.Mux
		err  error
	)

	When("single connection", func() {
		It("send and receive messages", func() {
			connCnt := 1
			msgCnt := 64

			lMux, pMux, err = connectMuxes()
			Expect(err).To(BeNil())
			Expect(lMux).ToNot(BeNil())
			Expect(pMux).ToNot(BeNil())

			lConn, pConn, err := openMuxes(lMux, pMux, connCnt)
			Expect(err).To(BeNil())
			Expect(len(lConn)).To(Equal(connCnt))
			Expect(len(pConn)).To(Equal(connCnt))

			sendAndReceive(lConn, pConn, msgCnt)
		})
	})

	When("multiple connections", func() {
		It("send and receive messages concurrently", func() {
			connCnt := 16
			msgCnt := 64

			lMux, pMux, err = connectMuxes()
			Expect(err).To(BeNil())
			Expect(lMux).ToNot(BeNil())
			Expect(pMux).ToNot(BeNil())

			lConn, pConn, err := openMuxes(lMux, pMux, connCnt)
			Expect(err).To(BeNil())
			Expect(len(lConn)).To(Equal(connCnt))
			Expect(len(pConn)).To(Equal(connCnt))

			sendAndReceive(lConn, pConn, msgCnt)
		})
	})
})

/*
// TODO
var _ = Describe("Read Queue Length", func() {
	var (
		lMux, pMux   mux.Mux
		connID       mux.ConnID
		lConn, pConn net.Conn
		err          error
		qLen         = 1
	)

	BeforeEach(func() {
		lMux, pMux, err = connectMuxes(mux.WithReadQueueLength(qLen))
		Expect(err).To(BeNil())
		Expect(lMux).ToNot(BeNil())
		Expect(pMux).ToNot(BeNil())

		connID = mux.LowestConnID
		lConn, err = lMux.Open(connID)
		Expect(err).To(BeNil())
		Expect(lConn).ToNot(BeNil())
		pConn, err = pMux.Open(connID)
		Expect(err).To(BeNil())
		Expect(pConn).ToNot(BeNil())
	})

	AfterEach(func() {
		if lMux != nil {
			lMux.Close()
		}
		if pMux != nil {
			pMux.Close()
		}
	})

	It("Messages get queued up till queue length", func() {
		var msg string

		// When
		for i := 0; i < qLen; i++ {
			msg = fmt.Sprintf("qlen test message #%d", i)
			_, err = lConn.Write([]byte(msg))
			Expect(err).To(BeNil())
		}

		// Then
		buf := make([]byte, len(msg))
		for i := 0; i < qLen; i++ {
			_, err = pConn.Read(buf)
			Expect(err).To(BeNil())
		}
	})

	It("Queue overflow closes mux, connections, results in read error", func() {
		var msg string

		// When
		for i := 0; i < qLen+1; i++ {
			msg = fmt.Sprintf("qlen test message #%d", i)
			_, err = lConn.Write([]byte(msg))
			Expect(err).To(BeNil())
		}

		// Then
		buf := make([]byte, len(msg))
		for i := 0; i < qLen; i++ {
			_, err = pConn.Read(buf)
		}
		_, err = pConn.Read(buf)
		Expect(err).ToNot(BeNil())

	})
})

var _ = Describe("Blocking and Unblocking", func() {
	// TODO...
})
*/

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

func sendAndReceive(lConn, pConn []net.Conn, msgCount int) {
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
				Expect(len(msg) <= maxMsg).To(BeTrue())
				messages = append(messages, msg)
			}
		}

		for _, msg = range messages {
			cnt, err = conn.Write([]byte(msg))
			Expect(err).To(BeNil())
			Expect(cnt).To(Equal(len(msg)))
		}

		cnt, err = conn.Write([]byte(endMsg))
		Expect(err).To(BeNil())
		Expect(cnt).To(Equal(len(endMsg)))

		return messages
	}

	// mesage receiver and collector
	read := func(conn net.Conn) []string {
		var (
			msg  = make([]byte, maxMsg)
			recv []string
			cnt  int
			err  error
		)

		for {
			cnt, err = conn.Read(msg)
			Expect(err).To(BeNil())
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
			Expect(sent).To(Equal(recv))
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

	// trigger senders/recevers and wait for them to finish
	close(start)
	wg.Wait()
}
