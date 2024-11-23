package core

import (
	"bft/2pac/network"
	"time"
)

type latencyMsg struct {
	msg *network.NetMessage
	ts  int64
}

type Transmitor struct {
	sender      *network.Sender
	receiver    *network.Receiver
	recvCh      chan ConsensusMessage
	msgCh       chan *network.NetMessage
	parameters  Parameters
	committee   Committee
	latencyChan []chan *latencyMsg
}

func NewTransmitor(
	node NodeID,
	sender *network.Sender,
	receiver *network.Receiver,
	parameters Parameters,
	committee Committee,
) *Transmitor {

	tr := &Transmitor{
		sender:      sender,
		receiver:    receiver,
		recvCh:      make(chan ConsensusMessage, 1_000),
		msgCh:       make(chan *network.NetMessage, 1_000),
		parameters:  parameters,
		committee:   committee,
		latencyChan: make([]chan *latencyMsg, len(parameters.Latency)),
	}

	for i := range tr.latencyChan {
		tr.latencyChan[i] = make(chan *latencyMsg, 1000)
		go func(ind int) {
			latency := tr.parameters.Latency[int(node)%len(parameters.Latency)][ind]
			for msg := range tr.latencyChan[ind] {
				now := time.Now().UnixMilli()
				if now-msg.ts >= int64(latency) {
					tr.msgCh <- msg.msg
				} else {
					time.Sleep(time.Duration(int64(latency)-now+msg.ts) * time.Millisecond)
					tr.msgCh <- msg.msg
				}
			}
		}(i)
	}

	go func() {
		for msg := range tr.msgCh {
			tr.sender.Send(msg)
		}
	}()

	go func() {
		for msg := range tr.receiver.RecvChannel() {
			tr.recvCh <- msg.(ConsensusMessage)
		}
	}()

	return tr
}

func (tr *Transmitor) Send(from, to NodeID, msg ConsensusMessage) error {
	var addr []string

	if to == NONE {
		addr = tr.committee.BroadCast(from)
	} else {
		addr = append(addr, tr.committee.Address(to))
	}

	// filter
	if tr.parameters.DDos && msg.MsgType() == ProposeType {
		time.AfterFunc(time.Millisecond*time.Duration(tr.parameters.NetwrokDelay), func() {
			tr.msgCh <- &network.NetMessage{
				Msg:     msg,
				Address: addr,
			}
		})
	} else {
		for _, address := range addr {
			node := tr.committee.IDByAddres(address)
			msg := &network.NetMessage{
				Msg:     msg,
				Address: []string{address},
			}
			tr.latencyChan[int(node)%len(tr.latencyChan)] <- &latencyMsg{
				msg: msg,
				ts:  time.Now().UnixMilli(),
			}
		}
	}

	return nil
}

func (tr *Transmitor) Recv() ConsensusMessage {
	return <-tr.recvCh
}

func (tr *Transmitor) RecvChannel() chan ConsensusMessage {
	return tr.recvCh
}
