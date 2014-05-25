package dtunnel

import (
	"bufio"
	"bytes"
	"errors"
	"github.com/vmihailenco/msgpack"
	"io"
	"log"
	"net"
	"time"
)

type MsgBuilder interface {
	MakeMsg(mt uint16, ct uint16, data []byte, flags uint16) *Msg
	MakeErrorMsg(err error, flags uint16) *Msg
}

//Read bytes from Msg channel
type TunnelReader struct {
	recvChan chan *Msg
	recvBuff []byte
	initMsg  *Msg
	cache    Cache
	isEof    bool
}

func (c *TunnelReader) readMsgFromChannel() (*Msg, error) {
	var msg *Msg
	if c.initMsg != nil {
		msg = c.initMsg
		c.initMsg = nil
	} else {
		msg = <-c.recvChan
	}

	if msg == nil {
		return nil, io.EOF
	}
	if msg.IsEndOfStream() {
		return msg, io.EOF
	}
	return msg, nil
}

func (c *TunnelReader) readFromChannel(b []byte) (n int, err error) {
	var msg *Msg
	for {
		msg, err = c.readMsgFromChannel()
		if msg == nil {
			return
		}
		if msg.GetMsgType() == ERROR {
			err = errors.New(msg.Body.String())
			return
		}
		if err == io.EOF {
			break
		}
		if msg.GetMsgType() == TCP_DATA && len(msg.Body.(*TcpData).GetPayload()) > 0 {
			break
		}
	}

	var payload []byte

	body := msg.Body.(*TcpData)
	if body.ContentType == CT_CACHE_DIFF {
		var cerr error
		diff := new(DiffContent)
		cerr = msgpack.Unmarshal(body.Payload, diff)
		if cerr != nil {
			return n, cerr
		}
		if len(diff.PatchTo) > 0 {
			payload, cerr = decompress(c.cache, diff)
			if cerr != nil {
				return n, cerr
			}
		} else {
			payload = diff.Diff
		}
	} else {
		payload = body.Payload
	}

	n = len(b)
	if len(payload) < n {
		n = len(payload)
	}
	copy(b[:n], payload[:n])
	c.recvBuff = append(c.recvBuff, payload[n:]...)

	return
}

func (c *TunnelReader) readFromBuff(b []byte) (n int) {
	n = len(b)
	if len(c.recvBuff) < n {
		n = len(c.recvBuff)
	}
	if n > 0 {
		copy(b[:n], c.recvBuff[:n])
		c.recvBuff = c.recvBuff[n:]
	}
	return n
}

func (c *TunnelReader) Read(b []byte) (n int, err error) {
	var cn int
	n = c.readFromBuff(b)
	if n < len(b) && !c.isEof {
		cn, err = c.readFromChannel(b[n:])
		n += cn
		if err == io.EOF {
			c.isEof = true
			err = nil
		}
	}
	if c.isEof && len(c.recvBuff) == 0 {
		err = io.EOF
	}
	return
}

func (c *TunnelReader) Close() error {
	return nil
}

type CachedTunnelReader struct {
	*TunnelReader
	cache    Cache
	cacheKey []byte
	buff     *bytes.Buffer
}

func (c *CachedTunnelReader) Read(b []byte) (n int, err error) {
	n, err = c.TunnelReader.Read(b)
	if n > 0 {
		c.buff.Write(b[:n])
	}
	return
}

func (c *CachedTunnelReader) Close() error {
	c.cache.Set(c.cacheKey, c.buff.Bytes())
	return c.TunnelReader.Close()
}

type TunnelWriter struct {
	sendChan chan *Msg
	msgMaker MsgBuilder
}

func (c *TunnelWriter) Write(b []byte) (n int, err error) {
	n = len(b)
	data := make([]byte, n, n)
	copy(data, b)
	c.sendChan <- c.msgMaker.MakeMsg(TCP_DATA, CT_RAW, data, 0)
	return
}

func (c *TunnelWriter) Close() error {
	c.sendChan <- c.msgMaker.MakeMsg(TCP_DATA, CT_RAW, []byte(""), FLAG_STREAM_END)
	return nil
}

type TimeoutWriter struct {
	bw           *bufio.Writer
	timeout      time.Duration
	flushPending bool
}

func (w *TimeoutWriter) Write(b []byte) (n int, err error) {
	n, err = w.bw.Write(b)
	if err == nil && !w.flushPending {
		w.flushPending = true
		go w.scheduleFlush()
	}
	return
}

func (w *TimeoutWriter) Flush() error {
	w.flushPending = false
	return w.bw.Flush()
}

func (w *TimeoutWriter) scheduleFlush() {
	select {
	case <-time.After(w.timeout):
		if w.flushPending && w.bw.Buffered() > 0 {
			w.bw.Flush()
		}
	}
	w.flushPending = false
}

// A CachedTunnelWriter will cache the data, send them with one msg when close
type CachedTunnelWriter struct {
	*TunnelWriter
	comp         Compressor
	buf          *bytes.Buffer
	noCache      bool
	maxCacheSize int
	maxDelay     time.Duration
	deadline     time.Time
}

func (c *CachedTunnelWriter) Write(b []byte) (n int, err error) {
	//init deadline
	if c.deadline.IsZero() {
		c.deadline = time.Now().Add(c.maxDelay)
	}

	//timeout
	if !c.noCache && (time.Now().After(c.deadline) || c.buf.Len()+len(b) > c.maxCacheSize) {
		err = c.cancelCache()
		if err != nil {
			return
		}
	}

	if c.noCache {
		return c.TunnelWriter.Write(b)
	} else {
		return c.buf.Write(b)
	}
}

func (c *CachedTunnelWriter) cancelCache() (err error) {
	c.noCache = true
	_, err = c.TunnelWriter.Write(c.buf.Bytes())
	return
}

func (c *CachedTunnelWriter) Close() (err error) {
	if c.noCache {
		c.TunnelWriter.sendChan <- c.msgMaker.MakeMsg(TCP_DATA, CT_RAW, []byte(""), FLAG_STREAM_END)
		return
	}
	_, err = c.comp.Write(c.buf.Bytes())
	if err != nil {
		return
	}
	err = c.comp.Close()
	if err != nil {
		return
	}
	c.TunnelWriter.sendChan <- c.msgMaker.MakeMsg(TCP_DATA, CT_CACHE_DIFF, c.comp.Bytes(), FLAG_STREAM_END)
	return
}

func NewCachedTunnelWriter(w *TunnelWriter, comp Compressor) *CachedTunnelWriter {
	cw := &CachedTunnelWriter{
		TunnelWriter: w,
		comp:         comp,
		maxCacheSize: MAX_CACHE_SIZE,
		maxDelay:     2 * time.Second, //if we can't receive all the data in 2 seconds , stream it
		buf:          new(bytes.Buffer),
	}
	return cw
}

// TunnelConn
type TunnelConn struct {
	io.Reader
	io.WriteCloser
}

func (c *TunnelConn) LocalAddr() net.Addr {
	return nil
}

func (c *TunnelConn) RemoteAddr() net.Addr {
	return nil
}

func (c *TunnelConn) SetDeadline(t time.Time) error {
	//c.readTimeout = t
	//c.writeTimeout = t
	return nil
}

func (c *TunnelConn) SetReadDeadline(t time.Time) error {
	//c.readTimeout = t
	return nil
}

func (c *TunnelConn) SetWriteDeadline(t time.Time) error {
	//c.writeTimeout = t
	return nil
}

func copyAndClose(w net.Conn, r io.Reader, finish chan bool) {
	connOk := true
	if _, err := io.Copy(w, r); err != nil {
		connOk = false
		log.Printf("Error copying to client %s %s", err, connOk)
	}
	if err := w.Close(); err != nil && connOk {
		log.Printf("Error closing %s", err)
	}
	finish <- true
}

func piping(src, dst net.Conn) {
	ch := make(chan bool)
	go copyAndClose(src, dst, ch)
	go copyAndClose(dst, src, ch)
	<-ch
	<-ch
	//make both direction is finished
}
