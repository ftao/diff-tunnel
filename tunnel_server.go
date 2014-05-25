package dtunnel

import (
	zmq "github.com/pebbe/zmq4"
	"log"
)

type TunnelServer struct {
	socket      *zmq.Socket
	repChan     chan *Msg
	httpWorker  Worker
	tcpWorker   Worker
	cacheWorker Worker
}

func NewTunnelServer(bind string) (*TunnelServer, error) {
	cm := makeCacheManager()
	socket, _ := zmq.NewSocket(zmq.ROUTER)
	socket.Bind(bind)
	repChan := make(chan *Msg, 10)
	return &TunnelServer{
		socket, repChan,
		NewMultiStreamHttpWorker(cm),
		NewMultiStreamTcpWorker(),
		NewCacheWorker(cm),
	}, nil
}

func NewTunnelServerKeyPair(bind string, pub string, secret string) (*TunnelServer, error) {
	if len(pub) == 0 || len(secret) == 0 {
		return NewTunnelServer(bind)
	}
	zmq.AuthCurveAdd("global", zmq.CURVE_ALLOW_ANY)
	cm := makeCacheManager()
	socket, _ := zmq.NewSocket(zmq.ROUTER)
	socket.ServerAuthCurve("global", secret)
	socket.Bind(bind)
	repChan := make(chan *Msg, 10)
	zmq.AuthStart()
	zmq.AuthCurveAdd(zmq.CURVE_ALLOW_ANY)
	return &TunnelServer{
		socket, repChan,
		NewMultiStreamHttpWorker(cm),
		NewMultiStreamTcpWorker(),
		NewCacheWorker(cm),
	}, nil
}

func (s *TunnelServer) Run() error {
	go s.httpWorker.Run(s.repChan)
	go s.tcpWorker.Run(s.repChan)
	go s.cacheWorker.Run(s.repChan)

	go func() {
		for msg := range s.repChan {
			frames, _ := toFrames(msg)
			log.Printf("[ts]send msg %s", msg)
			if msg.GetMsgType() == ERROR {
				log.Printf("error msg:%s", msg.Body.(*ErrorData).String())
			}
			s.socket.SendMessage(frames)
		}
	}()

	for {
		frames, err := s.socket.RecvMessageBytes(0)
		if err != nil {
			continue
		}
		msg, err := fromFrames(frames)
		if err != nil {
			log.Printf("[ts]invalid frames %s", err.Error())
			continue
		}
		log.Printf("[ts]recv msg %s", msg)

		if msg.GetMsgType() == CACHE_SHARE {
			s.cacheWorker.GetReqChannel() <- msg
			continue
		}
		if msg.TestFlag(FLAG_HTTP) {
			s.httpWorker.GetReqChannel() <- msg
		} else {
			s.tcpWorker.GetReqChannel() <- msg
		}
	}

	return nil
}

func (s *TunnelServer) Close() error {
	s.socket.Close()
	return nil
}
