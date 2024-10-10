package core

import (
	"bft/2pac/crypto"
	"bytes"
	"sync"
	"sync/atomic"
)

type SPB struct {
	corer    *Core
	Proposer NodeID
	View     int

	hMutex    sync.Mutex
	preQc     *BlockQC
	blockHash [2]crypto.Digest
	blockQcs  [2]*BlockQC

	uMutex          sync.Mutex
	unHandlePropose []*ProposeMsg
	unHandleFinVote []*FinVoteMsg

	voteNums      [2]atomic.Int32
	speedVoteNums atomic.Int32
	isSpeedCert   atomic.Bool
}

func NewSPB(corer *Core, Proposer NodeID, View int) *SPB {
	instance := &SPB{
		corer:    corer,
		Proposer: Proposer,
		View:     View,
	}
	return instance
}

func (s *SPB) processProposeMsg(p *ProposeMsg) {
	if p.Height == HEIGHT_1 {
		s.hMutex.Lock()
		s.preQc = p.B.Qc
		s.blockHash[HEIGHT_1] = p.B.Hash()
		s.hMutex.Unlock()

		s.uMutex.Lock()
		for _, p := range s.unHandlePropose {
			go s.processProposeMsg(p)
		}
		s.uMutex.Unlock()
	} else if p.Height == HEIGHT_2 {
		s.hMutex.Lock()
		if hash := s.blockHash[HEIGHT_1]; !bytes.Equal(hash[:], p.B.Qc.ParentBlockHash[:]) {
			s.uMutex.Lock()
			s.unHandlePropose = append(s.unHandlePropose, p)
			s.uMutex.Unlock()
			s.hMutex.Unlock()
			return
		}
		s.blockQcs[HEIGHT_1] = p.B.Qc
		s.blockHash[HEIGHT_2] = p.B.Hash()
		s.hMutex.Unlock()

		s.uMutex.Lock()
		for _, fv := range s.unHandleFinVote {
			go s.processFinVoteMsg(fv)
		}
		s.uMutex.Unlock()
	}

	//make vote
	vote, _ := NewVoteMsg(s.corer.nodeID, p.B, s.corer.sigService)
	if s.Proposer != s.corer.nodeID {
		s.corer.transmitor.Send(s.corer.nodeID, s.Proposer, vote)
	} else {
		s.corer.transmitor.RecvChannel() <- vote
	}
}

func (s *SPB) processVoteMsg(v *VoteMsg) {
	nums := s.voteNums[v.Height].Add(1)
	if nums == int32(s.corer.committee.HightThreshold()) { //2f+1?
		qc := &BlockQC{
			View:            s.View,
			Height:          v.Height,
			Proposer:        v.Proposer,
			ParentBlockHash: v.BlockHash,
			Shares:          make(map[NodeID]crypto.SignatureShare),
		}
		if v.Height == HEIGHT_1 {
			//new propose
			if block := s.corer.generatorBlock(v.View, HEIGHT_2, qc); block != nil {
				propose, _ := NewProposeMsg(block, s.corer.sigService)
				s.corer.transmitor.Send(s.corer.nodeID, NONE, propose)
				s.corer.transmitor.RecvChannel() <- propose
			}
		} else {
			//fin Vote
			finVote, _ := NewFinVoteMsg(s.corer.nodeID, s.View, qc, s.corer.sigService)
			s.corer.transmitor.Send(s.corer.nodeID, NONE, finVote)
			s.corer.transmitor.RecvChannel() <- finVote
		}

	}
}

func (s *SPB) processFinVoteMsg(fv *FinVoteMsg) {
	if fv.Author != s.Proposer || fv.View != s.View {
		return
	}
	s.hMutex.Lock()
	if hash := s.blockHash[HEIGHT_2]; !bytes.Equal(hash[:], fv.Qc.ParentBlockHash[:]) {
		s.uMutex.Lock()
		s.unHandleFinVote = append(s.unHandleFinVote, fv)
		s.uMutex.Unlock()
		s.hMutex.Unlock()
		return
	}
	s.blockQcs[HEIGHT_2] = fv.Qc
	s.hMutex.Unlock()
	sv, _ := NewSpeedVoteMsg(s.corer.nodeID, fv.Qc, s.corer.sigService)
	if s.Proposer == s.corer.nodeID {
		s.corer.transmitor.RecvChannel() <- sv
	} else {
		s.corer.transmitor.Send(s.corer.nodeID, s.Proposer, sv)
	}
}

func (s *SPB) processSpeedVoteMsg(sv *SpeedVoteMsg) {
	if sv.Proposer != s.Proposer || sv.View != s.View {
		return
	}
	nums := s.speedVoteNums.Add(1)
	if nums == int32(s.corer.committee.HightThreshold()) {
		s.isSpeedCert.Store(true)
	}
}

func (s *SPB) SpeedCert() bool {
	return s.isSpeedCert.Load()
}

func (s *SPB) DecCert() bool {
	s.hMutex.Lock()
	defer s.hMutex.Unlock()
	return s.blockQcs[HEIGHT_1] != nil && s.blockQcs[HEIGHT_2] != nil
}

func (s *SPB) BlockHash(height uint8) crypto.Digest {
	s.hMutex.Lock()
	defer s.hMutex.Unlock()
	if height > HEIGHT_2 {
		return crypto.Digest{}
	}
	return s.blockHash[height]
}

func (s *SPB) QCLEvel() (uint8, *BlockQC) {
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
