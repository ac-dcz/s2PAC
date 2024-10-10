package core

import (
	"bft/2pac/crypto"
)

type Aggregator struct {
	elects []*ElectMsg
	used   map[NodeID]struct{}
}

func (a *Aggregator) Append(elect *ElectMsg, committee Committee, sigService *crypto.SigService) (NodeID, error) {
	if _, ok := a.used[elect.Author]; ok {
		return NONE, ErrUsedElect(ElectType, elect.View, int(elect.Author))
	} else {
		a.used[elect.Author] = struct{}{}
		a.elects = append(a.elects, elect)
		if len(a.elects) == committee.HightThreshold() {
			var shares []crypto.SignatureShare
			for _, e := range a.elects {
				shares = append(shares, e.SigShare)
			}
			qc, err := crypto.CombineIntactTSPartial(shares, sigService.ShareKey, elect.Hash())
			if err != nil {
				return NONE, err
			}
			var randint NodeID = 0
			for i := 0; i < 4; i++ {
				randint = randint<<8 + NodeID(qc[i])
			}
			return randint % NodeID(committee.Size()), nil
		}
	}
	return NONE, nil
}

type Elector struct {
	leaders    map[int]NodeID
	aggregator map[int]*Aggregator
	sigService *crypto.SigService
	committee  Committee
}

func NewElector(sigService *crypto.SigService, committee Committee) *Elector {
	return &Elector{
		leaders:    make(map[int]NodeID),
		aggregator: make(map[int]*Aggregator),
		sigService: sigService,
		committee:  committee,
	}
}

func (e *Elector) Add(elect *ElectMsg) (NodeID, error) {
	a, ok := e.aggregator[elect.View]
	if !ok {
		a = &Aggregator{
			used: map[NodeID]struct{}{},
		}
		e.aggregator[elect.View] = a
	}
	leader, err := a.Append(elect, e.committee, e.sigService)
	if err == nil && leader != NONE {
		e.leaders[elect.View] = leader
	}
	return leader, err
}
