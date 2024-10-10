package core

import (
	"bft/2pac/crypto"
	"bytes"
	"sync"
	"sync/atomic"
)

type DAGBigSPB struct {
	corer    *DAGBigCore
	Proposer NodeID
	View     int

	hMutex    sync.Mutex
	preQc     *BlockQC
	blockHash [2]crypto.Digest
	blockQcs  [2]*BlockQC

	uMutex            sync.Mutex
	unHandlePropose   []*ProposeMsg
	unHandleVote      [2][]*VoteMsg
	unHandleSpeedVote []*SpeedVoteMsg

	voteNums      [2]atomic.Int32
	speedVoteNums atomic.Int32
	isSpeedCert   atomic.Bool

	callBack chan *DAGBigSPB
}

func NewDAGBigSPB(corer *DAGBigCore, Proposer NodeID, View int, callBack chan *DAGBigSPB) *DAGBigSPB {
	instance := &DAGBigSPB{
		corer:    corer,
		Proposer: Proposer,
		View:     View,
		callBack: callBack,
	}
	return instance
}

func (s *DAGBigSPB) processProposeMsg(p *ProposeMsg) {
	if p.Height == HEIGHT_1 {
		s.hMutex.Lock()
		s.preQc = p.B.Qc
		s.blockHash[HEIGHT_1] = p.B.Hash()
		s.hMutex.Unlock()

		s.uMutex.Lock()
		for _, p := range s.unHandlePropose {
			go s.processProposeMsg(p)
		}
		for _, vote := range s.unHandleVote[HEIGHT_1] {
			go s.processVoteMsg(vote)
		}
		s.uMutex.Unlock()
	} else if p.Height == HEIGHT_2 {

		s.hMutex.Lock()
		// if hash := s.blockHash[HEIGHT_1]; !bytes.Equal(hash[:], p.B.Qc.ParentBlockHash[:]) {
		// 	s.uMutex.Lock()
		// 	s.unHandlePropose = append(s.unHandlePropose, p)
		// 	s.uMutex.Unlock()
		// 	s.hMutex.Unlock()
		// 	return
		// }
		s.blockHash[HEIGHT_2] = p.B.Hash()
		s.hMutex.Unlock()

		s.uMutex.Lock()

		for _, vote := range s.unHandleVote[HEIGHT_2] {
			go s.processVoteMsg(vote)
		}

		s.uMutex.Unlock()
	}

	//make vote
	vote, _ := NewVoteMsg(s.corer.nodeID, p.B, s.corer.sigService)
	s.corer.transmitor.Send(s.corer.nodeID, NONE, vote)
	s.corer.transmitor.RecvChannel() <- vote
}

func (s *DAGBigSPB) processVoteMsg(v *VoteMsg) {
	s.hMutex.Lock()
	if hash := s.blockHash[v.Height]; !bytes.Equal(hash[:], v.BlockHash[:]) {
		s.unHandleVote[v.Height] = append(s.unHandleVote[v.Height], v)
		s.hMutex.Unlock()
		return
	}
	s.hMutex.Unlock()

	nums := s.voteNums[v.Height].Add(1)
	if nums == int32(s.corer.committee.HightThreshold()) { //2f+1?
		qc := &BlockQC{
			View:            s.View,
			Height:          v.Height,
			Proposer:        v.Proposer,
			ParentBlockHash: v.BlockHash,
			Shares:          make(map[NodeID]crypto.SignatureShare),
		}
		s.hMutex.Lock()
		s.blockQcs[v.Height] = qc
		s.hMutex.Unlock()
		if v.Height == HEIGHT_1 {
			// //new propose
			// if block := s.corer.generatorBlock(v.View, HEIGHT_2, qc); block != nil {
			// 	propose, _ := NewProposeMsg(block, s.corer.sigService)
			// 	s.corer.transmitor.Send(s.corer.nodeID, NONE, propose)
			// 	s.corer.transmitor.RecvChannel() <- propose
			// }
			s.callBack <- s
		} else if v.Height == HEIGHT_2 {
			sv, _ := NewSpeedVoteMsg(s.corer.nodeID, qc, s.corer.sigService)
			s.corer.transmitor.Send(s.corer.nodeID, NONE, sv)
			s.corer.transmitor.RecvChannel() <- sv

			s.uMutex.Lock()
			for _, sv := range s.unHandleSpeedVote {
				go s.processSpeedVoteMsg(sv)
			}
			s.uMutex.Unlock()

		}
	}
}

func (s *DAGBigSPB) processSpeedVoteMsg(sv *SpeedVoteMsg) {
	if sv.Proposer != s.Proposer || sv.View != s.View {
		return
	}

	s.hMutex.Lock()
	if s.blockQcs[HEIGHT_2] == nil {
		s.unHandleSpeedVote = append(s.unHandleSpeedVote, sv)
		s.hMutex.Unlock()
		return
	}
	s.hMutex.Unlock()

	nums := s.speedVoteNums.Add(1)
	if nums == int32(s.corer.committee.HightThreshold()) {
		s.isSpeedCert.Store(true)
	}
}

func (s *DAGBigSPB) SpeedCert() bool {
	return s.isSpeedCert.Load()
}

func (s *DAGBigSPB) DecCert() bool {
	s.hMutex.Lock()
	defer s.hMutex.Unlock()
	return s.blockQcs[HEIGHT_1] != nil && s.blockQcs[HEIGHT_2] != nil
}

func (s *DAGBigSPB) BlockHash(height uint8) crypto.Digest {
	s.hMutex.Lock()
	defer s.hMutex.Unlock()
	if height > HEIGHT_2 {
		return crypto.Digest{}
	}
	return s.blockHash[height]
}

func (s *DAGBigSPB) GetBlockQC(height uint8) *BlockQC {
	s.hMutex.Lock()
	defer s.hMutex.Unlock()
	if height > HEIGHT_2 {
		return nil
	}
	return s.blockQcs[height]
}

func (s *DAGBigSPB) QCLEvel() (uint8, *BlockQC) {
	s.hMutex.Lock()
	defer s.hMutex.Unlock()
	if s.blockQcs[HEIGHT_1] != nil && s.blockQcs[HEIGHT_2] != nil {
		return LEVEL_2, s.blockQcs[HEIGHT_2]
	} else if s.blockQcs[HEIGHT_1] != nil {
		return LEVEL_1, s.blockQcs[HEIGHT_1]
	} else {
		return LEVEL_0, s.preQc
	}
}
