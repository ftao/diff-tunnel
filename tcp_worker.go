package dtunnel

import (
	"errors"
	"log"
	"net"
)

type TcpWorker struct {
	reqChan chan *Msg
}

func (w *TcpWorker) GetReqChannel() chan *Msg {
	return w.reqChan
}

func (w *TcpWorker) Run(repChan chan *Msg) error {
	connectMsg := <-w.reqChan
	if connectMsg.GetMsgType() != TCP_CONNECT {
		return errors.New("invalid first msg")
	}

	msgMaker := NewMsgBuilderFromMsg(connectMsg)
	conn, err := w.handleConnect(connectMsg, repChan, msgMaker)
	if err != nil {
		return err
	}

	defer conn.Close()
	tunnelConn := &TunnelConn{
		&TunnelReader{recvChan: w.reqChan},
		&TunnelWriter{
			sendChan: repChan,
			msgMaker: msgMaker,
		},
	}
	piping(tunnelConn, conn)
	return nil
}

func (w *TcpWorker) handleConnect(reqMsg *Msg, repChan chan *Msg, msgMaker MsgBuilder) (conn net.Conn, err error) {
	host := string(reqMsg.Body.(*TcpData).GetPayload())
	conn, err = net.Dial("tcp", host)
	if err != nil {
		repChan <- msgMaker.MakeErrorMsg(err, 0)
	} else {
		remoteAddr := conn.RemoteAddr().String()
		log.Printf("dial to %s success, remote: %s", host, remoteAddr)
		repChan <- msgMaker.MakeMsg(TCP_CONNECT_REP, CT_RAW, []byte(remoteAddr), FLAG_STREAM_BEGIN)
	}
	return
}

type TcpWorkerFactory struct{}

func (s *TcpWorkerFactory) MakeStreamWorker(sid UID) Worker {
	return &TcpWorker{make(chan *Msg)}
}

func NewMultiStreamTcpWorker() Worker {
	return &MultiStreamWorker{
		factory: new(TcpWorkerFactory),
		workers: make(map[UID]Worker),
		reqChan: make(chan *Msg),
	}
}
