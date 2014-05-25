package dtunnel

import (
	"bufio"
	"log"
	"net/http"
	"time"
)

const (
	MAX_CACHE_SIZE int = 5 * 1024 * 1024
	MAX_BUFF_SIZE  int = 500 * 1024
)

type HttpWorker struct {
	reqChan chan *Msg
	ht      *http.Transport
	cm      *CacheManager
}

func (w *HttpWorker) GetReqChannel() chan *Msg {
	return w.reqChan
}

func (w *HttpWorker) Run(repChan chan *Msg) error {
	firstMsg := <-w.reqChan
	msgMaker := NewMsgBuilderFromMsg(firstMsg)

	var err error
	defer func() {
		if err != nil {
			repChan <- msgMaker.MakeErrorMsg(err, 0)
		}
	}()

	reader := &TunnelReader{recvChan: w.reqChan, initMsg: firstMsg}
	req, err := http.ReadRequest(bufio.NewReader(reader))
	if err != nil {
		log.Printf("read request errror: %v", err)
		return err
	}
	resp, err := w.ht.RoundTrip(req)
	if err != nil {
		log.Printf("round trip errror: %v", err)
		return err
	}

	cacheAble := resp.ContentLength < int64(MAX_CACHE_SIZE)

	writer := &TunnelWriter{
		sendChan: repChan,
		msgMaker: msgMaker,
	}
	if cacheAble {
		cacheKey := makeCacheKey(req)
		digest, _ := w.cm.GetPeerDigest("global", cacheKey)
		cwriter := NewCachedTunnelWriter(writer, NewCacheCompressor(w.cm.local, cacheKey, digest, true))
		resp.Write(cwriter)
		cwriter.Close()
	} else {
		log.Printf("result is not cacheable, content-length %d", resp.ContentLength)
		bw := &TimeoutWriter{bw: bufio.NewWriterSize(writer, MAX_BUFF_SIZE), timeout: 10 * time.Millisecond}
		resp.Write(bw)
		bw.Flush()
		writer.Close()
	}
	return nil
}

type HttpWorkerFactory struct {
	cm *CacheManager
}

func (s *HttpWorkerFactory) MakeStreamWorker(sid UID) Worker {
	return &HttpWorker{make(chan *Msg, 10), new(http.Transport), s.cm}
}

func NewMultiStreamHttpWorker(cm *CacheManager) Worker {
	return &MultiStreamWorker{
		factory: &HttpWorkerFactory{cm: cm},
		workers: make(map[UID]Worker),
		reqChan: make(chan *Msg),
	}
}
