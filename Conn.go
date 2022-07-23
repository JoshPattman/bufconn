package bufconn

import (
	"errors"
	"net"
	"time"
)

// Conn is a wrapper for a net.Conn which provides more high level functionality.
// It allows concurrent safe communication over a socket with a more logical API.
// To do this, set the message handler (which is what is called when a new message comes in), or queue an operation.
// For example, to sned hello to the remote, you could queue an operation which writes hello to the socket. You cannot directly write to the socket to prevent multiple goroutines writing at the same time and interfering
type Conn struct {
	netconn    net.Conn
	readBuf    []byte
	readChan   chan byte
	opChan     chan func(*C)
	msgHandler func(*C)
	msgDelim   byte
	isStopped  bool
	stopChan   chan bool
}

// NewConn creates a new Conn using a net.Conn, a new message handler function, and a delimeter for messages
func NewConn(c net.Conn, handler func(*C), delim byte) *Conn {
	if handler == nil {
		handler = func(c *C) {
			c.ReadMsg(0)
		}
	}
	conn := &Conn{
		c,
		make([]byte, 0),
		make(chan byte, 100),
		make(chan func(*C), 10),
		handler,
		delim,
		false,
		make(chan bool, 10),
	}
	go func() {
		minibuf := make([]byte, 1)
		for {
			// Check if the conn has been stopped. If the exit is not clean (i.e. remote simply stops responding) then this goroutine will hang forever
			if conn.isStopped {
				return
			}
			_, err := conn.netconn.Read(minibuf)
			if err != nil {
				conn.Stop()
				return
			}
			conn.readChan <- minibuf[0]
		}
	}()
	go func() {
		for {
			// We do this to give stop priority over other waiting operations
			if len(conn.stopChan) > 0 {
				<-conn.stopChan
				conn.netconn.Close()
				return
			}
			select {
			case b := <-conn.readChan:
				conn.readBuf = append(conn.readBuf, b)
				if b == conn.msgDelim {
					conn.msgHandler(&C{conn})
				}
			case o := <-conn.opChan:
				o(&C{conn})
			case <-conn.stopChan:
				conn.netconn.Close()
				return
			}
		}
	}()
	return conn
}

func (c *Conn) updateWholeBuffer() {
	for len(c.readChan) > 0 {
		c.readBuf = append(c.readBuf, <-c.readChan)
	}
}

// QueueOperation adds an operation to the end of the queue of operations, and it will be performed when possible
func (c *Conn) QueueOperation(o func(*C)) {
	c.opChan <- o
}

// SetMessageHandler changes the message handler for the next message. It will come into effect after the current operation or handler
func (c *Conn) SetMessageHandler(f func(*C)) {
	if f == nil {
		f = func(c *C) {
			c.ReadMsg(0)
		}
	}
	c.msgHandler = f
}

// Stop will exit cleanly by finishing the current operation first
func (c *Conn) Stop() {
	if c.isStopped {
		return
	}
	c.isStopped = true
	c.stopChan <- true
}

// IsStopped checks if the connection will run any further operations. This may return true (stopped) even if an operation is currently ongoing
func (c *Conn) IsStopped() bool {
	return c.isStopped
}

// Underlying net.Conn.LocalAddr()
func (c *Conn) LocalAddr() net.Addr {
	return c.netconn.LocalAddr()
}

// Underlying net.Conn.RemoteAddr()
func (c *Conn) RemoteAddr() net.Addr {
	return c.netconn.RemoteAddr()
}

// C is a wrapper for Conn which adds the ability to read and write messages. This should only be used within message handlers and operations
type C struct {
	*Conn
}

// ReadMsg reads an entire message (string ending with the delimer) from the buffer. It will wait for it to become available.
// If the timeout is reached, this function will return an error. If the timeout is zero, then no timeout will be used.
// It does NOT include the delimeter in the return
func (c *C) ReadMsg(timeout time.Duration) (string, error) {
	now := time.Now()
	for {
		if time.Since(now) > timeout && timeout != 0 {
			return "", errors.New("message read timeout")
		}
		c.Conn.updateWholeBuffer()
		for i, b := range c.Conn.readBuf {
			if b == c.Conn.msgDelim {
				out := make([]byte, i+1)
				copy(out, c.Conn.readBuf)
				c.Conn.readBuf = c.Conn.readBuf[i+1:]
				return string(out[:len(out)-1]), nil
			}
		}
	}
}

// Read reads an number of bytes from the buffer. It will wait for them to become available.
// If the timeout is reached, this function will return an error. If the timeout is zero, then no timeout will be used
func (c *C) Read(n int, timeout time.Duration) ([]byte, error) {
	now := time.Now()
	for {
		if time.Since(now) > timeout && timeout != 0 {
			return []byte{}, errors.New("message read timeout")
		}
		c.Conn.updateWholeBuffer()
		if len(c.Conn.readBuf) >= n {
			out := make([]byte, n)
			copy(out, c.Conn.readBuf)
			c.Conn.readBuf = c.Conn.readBuf[n:]
			return out, nil
		}
	}
}

// Write writes a slice of bytes to the underlying net.Conn. It returns the number of bytes written and the error
func (c *C) Write(bs []byte) (int, error) {
	return c.Conn.netconn.Write(bs)
}

// WriteMsg takes a string message and appends the delimeter, then writes it to the underlying connection
func (c *C) WriteMsg(msg string) (int, error) {
	return c.Write(append([]byte(msg), c.Conn.msgDelim))
}
