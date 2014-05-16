package main

import (
	_ "log"
)

type Worker interface {
	GetReqChannel() chan *Msg
	Run(repChan chan *Msg) error
}

type StreamWorkerMaker interface {
	MakeStreamWorker(sid UID) Worker
}

type MultiStreamWorker struct {
	factory StreamWorkerMaker
	workers map[UID]Worker
	reqChan chan *Msg
}

func (w *MultiStreamWorker) GetReqChannel() chan *Msg {
	return w.reqChan
}

func (w *MultiStreamWorker) Run(repChan chan *Msg) error {
	for msg := range w.reqChan {
		w.handleMsg(msg, repChan)
	}
	return nil
}

func (w *MultiStreamWorker) handleMsg(msg *Msg, repChan chan *Msg) {
	sid := msg.GetStreamId()
	worker, ok := w.workers[sid]
	if !ok {
		worker = w.factory.MakeStreamWorker(sid)
		w.workers[sid] = worker
		go worker.Run(repChan)
	}
	worker.GetReqChannel() <- msg
}
