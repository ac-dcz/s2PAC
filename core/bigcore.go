package core

import (
	"bft/2pac/crypto"
	"bft/2pac/logger"
	"bft/2pac/pool"
	"bft/2pac/store"
)

type BigCore struct {
	nodeID        NodeID
	view          int
	height        uint8
	committee     Committee
	parameters    Parameters
	txpool        *pool.Pool
	transmitor    *Transmitor
	sigService    *crypto.SigService
	store         *store.Store
	eletor        *Elector
	commitor      *Commitor
	spbInstances  map[int]map[NodeID]*BigSPB
	speedVoteNums map[int]map[NodeID]struct{}
	reportNums    map[int][3]int
}

func NewBigCore(
	nodeID NodeID,
	committee Committee,
	parameters Parameters,
	txpool *pool.Pool,
	transmitor *Transmitor,
	store *store.Store,
	sigService *crypto.SigService,
	commitChannel chan<- struct{},
) *BigCore {

	corer := &BigCore{
		nodeID:        nodeID,
		committee:     committee,
		view:          0,
		height:        HEIGHT_1,
		parameters:    parameters,
		txpool:        txpool,
		transmitor:    transmitor,
		sigService:    sigService,
		store:         store,
		spbInstances:  make(map[int]map[NodeID]*BigSPB),
		speedVoteNums: make(map[int]map[NodeID]struct{}),
		reportNums:    make(map[int][3]int),
		eletor:        NewElector(sigService, committee),
		commitor:      NewCommitor(store, commitChannel),
	}

	return corer
}

func (corer *BigCore) getSpbInstance(view int, node NodeID) *BigSPB {
	instances, ok := corer.spbInstances[view]
	if !ok {
		instances = make(map[NodeID]*BigSPB)
		corer.spbInstances[view] = instances
	}
	instance, ok := instances[node]
	if !ok {
		instance = NewBigSPB(corer, node, view)
		instances[node] = instance
	}
	return instance
}

/*********************************Protocol***********************************************/
func (corer *BigCore) generatorBlock(view int, height uint8, qc *BlockQC) *Block {
	logger.Debug.Printf("procesing generatorBlock view %d height %d\n", view, height)
	// if view < corer.view || (view == corer.view && height < corer.height) {
	// 	return nil
	// }
	block := &Block{
		Author: corer.nodeID,
		View:   view,
		Height: height,
		Qc:     qc,
		Batch:  corer.txpool.GetBatch(),
	}
	if block.Batch.ID != -1 {
		//BenchMark Log
		logger.Info.Printf("create Block view %d height %d node %d batch_id %d \n", block.View, block.Height, block.Author, block.Batch.ID)
	}

	return block
}

func (corer *BigCore) handleProposeMsg(p *ProposeMsg) error {
	logger.Debug.Printf("Processing ProposeMsg Proposer %d View %d Height %d\n", p.Author, p.View, p.Height)
	if !p.Verify(corer.committee) {
		return ErrSignature(p.MsgType(), p.View, int(p.Author))
	}
	//Step1: check qc
	// if ok := p.B.Qc.Verify(corer.sigService); ok {

	// }

	//Step2: store block ? you should check that p.B.qc->B1 store
	if err := storeBlock(corer.store, p.B); err != nil {
		return err
	}

	go corer.getSpbInstance(p.View, p.Author).processProposeMsg(p)

	return nil
}

func (corer *BigCore) handleVoteMsg(v *VoteMsg) error {
	logger.Debug.Printf("Processing VoteMsg Author %d Proposer %d View %d Height %d\n", v.Author, v.Proposer, v.View, v.Height)
	if !v.Verify(corer.committee) {
		return ErrSignature(v.MsgType(), v.View, int(v.Author))
	}

	go corer.getSpbInstance(v.View, v.Proposer).processVoteMsg(v)

	return nil
}

func (corer *BigCore) handleSpeedVoteMsg(sv *SpeedVoteMsg) error {
	logger.Debug.Printf("Processing SpeedVoteMsg Author %d Proposer %d View %d\n", sv.Author, sv.Proposer, sv.View)
	if !sv.Verify(corer.committee) {
		return ErrSignature(sv.MsgType(), sv.View, int(sv.Author))
	}

	nodes, ok := corer.speedVoteNums[sv.View]
	if !ok {
		nodes = make(map[NodeID]struct{})
		corer.speedVoteNums[sv.View] = nodes
	}
	nodes[sv.Author] = struct{}{}

	if len(nodes) == corer.committee.HightThreshold() {
		//elect msg
		elect, _ := NewElectMsg(corer.nodeID, sv.View, corer.sigService)
		corer.transmitor.Send(corer.nodeID, NONE, elect)
		corer.transmitor.RecvChannel() <- elect
	}

	go corer.getSpbInstance(sv.View, sv.Author).processSpeedVoteMsg(sv)

	return nil
}

func (corer *BigCore) handleElectMsg(e *ElectMsg) error {
	logger.Debug.Printf("Processing ElectMsg Author %d View %d\n", e.Author, e.View)
	if !e.Verify() {
		return ErrSignature(e.MsgType(), e.View, int(e.Author))
	}

	if leader, err := corer.eletor.Add(e); err != nil {
		return err
	} else if leader != NONE {
		return corer.processCoin(leader, e.View)
	}

	return nil
}

func (corer *BigCore) processCoin(leader NodeID, view int) error {
	logger.Debug.Printf("Processing CoinLeader Leader %d View %d\n", leader, view)

	spb := corer.getSpbInstance(view, leader)
	if spb.SpeedCert() { //Commit height-2
		blockHash := spb.BlockHash(HEIGHT_2)
		if block, err := getBlock(corer.store, blockHash); err != nil {
			return err
		} else if block != nil {
			corer.commitor.Commit(block)
		}
	} else if spb.DecCert() { //Commit height-1
		blockHash := spb.BlockHash(HEIGHT_1)
		if block, err := getBlock(corer.store, blockHash); err != nil {
			return err
		} else if block != nil {
			corer.commitor.Commit(block)
		}
	}

	level, qc := spb.QCLEvel()

	report, _ := NewReportMsg(corer.nodeID, view, level, qc, corer.sigService)
	corer.transmitor.Send(corer.nodeID, NONE, report)
	corer.transmitor.RecvChannel() <- report

	return nil
}

func (corer *BigCore) handleReportMsg(r *ReportMsg) error {
	logger.Debug.Printf("Processing ReportMsg Author %d View %d Level %d\n", r.Author, r.View, r.Level)
	if !r.Verify(corer.committee) {
		return ErrSignature(r.MsgType(), r.View, int(r.Author))
	}
	nums := corer.reportNums[r.View]
	nums[r.Level]++
	corer.reportNums[r.View] = nums
	if nums[LEVEL_2] == 1 || nums[LEVEL_1] == corer.committee.HightThreshold() || nums[LEVEL_0] == corer.committee.HightThreshold() {
		corer.advanceView(r.View+1, r.BlockQc)
	}

	return nil
}

func (corer *BigCore) advanceView(view int, qc *BlockQC) {
	if view <= corer.view {
		return
	}
	logger.Debug.Printf("advance next view %d\n", view)
	corer.view = view
	block := corer.generatorBlock(view, HEIGHT_1, qc)
	if block == nil {
		panic("block is empty")
	}
	if propose, err := NewProposeMsg(block, corer.sigService); err != nil {
		logger.Error.Println(err)
		panic(err)
	} else {
		corer.transmitor.Send(corer.nodeID, NONE, propose)
		corer.transmitor.RecvChannel() <- propose
	}
}

func (corer *BigCore) Run() {
	if corer.nodeID >= NodeID(corer.parameters.Faults) {
		//first propose
		if block := corer.generatorBlock(corer.view, corer.height, nil); block == nil {
			panic("first block error")
		} else {
			if propose, err := NewProposeMsg(block, corer.sigService); err != nil {
				logger.Error.Println(err)
				panic(err)
			} else {
				corer.transmitor.Send(corer.nodeID, NONE, propose)
				corer.transmitor.RecvChannel() <- propose
			}

		}

		for msg := range corer.transmitor.RecvChannel() {
			var err error

			switch r := msg.(type) {
			case *ProposeMsg:
				err = corer.handleProposeMsg(r)
			case *VoteMsg:
				err = corer.handleVoteMsg(r)
			case *SpeedVoteMsg:
				err = corer.handleSpeedVoteMsg(r)
			case *ElectMsg:
				err = corer.handleElectMsg(r)
			case *ReportMsg:
				err = corer.handleReportMsg(r)
			}

			if err != nil {
				logger.Warn.Println(err)
			}

		}
	}
}
