package core

import (
	"bft/2pac/crypto"
	"bft/2pac/logger"
	"bft/2pac/pool"
	"bft/2pac/store"
	"fmt"
)

type DAGBigCore struct {
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
	commitor      *DAGCommitor
	weakVotes     map[int]map[NodeID]int
	spbInstances  map[int]map[NodeID]*DAGBigSPB
	speedVoteNums map[int]map[NodeID]int
	reportNums    map[int][3]int
	callBackSpb   chan *DAGBigSPB
}

func NewDAGBigCore(
	nodeID NodeID,
	committee Committee,
	parameters Parameters,
	txpool *pool.Pool,
	transmitor *Transmitor,
	store *store.Store,
	sigService *crypto.SigService,
	commitChannel chan<- struct{},
) *DAGBigCore {

	corer := &DAGBigCore{
		nodeID:        nodeID,
		committee:     committee,
		view:          0,
		height:        HEIGHT_1,
		parameters:    parameters,
		txpool:        txpool,
		transmitor:    transmitor,
		sigService:    sigService,
		store:         store,
		weakVotes:     make(map[int]map[NodeID]int),
		spbInstances:  make(map[int]map[NodeID]*DAGBigSPB),
		speedVoteNums: make(map[int]map[NodeID]int),
		reportNums:    make(map[int][3]int),
		eletor:        NewElector(sigService, committee),
		commitor:      NewDAGCommitor(store, commitChannel),
		callBackSpb:   make(chan *DAGBigSPB, 100),
	}

	return corer
}

func (corer *DAGBigCore) getSpbInstance(view int, node NodeID) *DAGBigSPB {
	instances, ok := corer.spbInstances[view]
	if !ok {
		instances = make(map[NodeID]*DAGBigSPB)
		corer.spbInstances[view] = instances
	}
	instance, ok := instances[node]
	if !ok {
		instance = NewDAGBigSPB(corer, node, view, corer.callBackSpb)
		instances[node] = instance
	}
	return instance
}

func (corer *DAGBigCore) getReference(view int, height uint8) []*BlockQC {
	var references []*BlockQC
	for i := 0; i < corer.committee.Size(); i++ {
		spb := corer.getSpbInstance(view, NodeID(i))
		if qc := spb.GetBlockQC(height); qc != nil {
			references = append(references, qc)
		}
	}
	return references
}

func (corer *DAGBigCore) addWeakVotes(view int, node NodeID) {
	votes, ok := corer.weakVotes[view]
	if !ok {
		votes = make(map[NodeID]int)
		corer.weakVotes[view] = votes
	}
	votes[node]++
}

func (corer *DAGBigCore) getWeakVotes(view int, node NodeID) int {
	votes, ok := corer.weakVotes[view]
	if !ok {
		votes = make(map[NodeID]int)
		corer.weakVotes[view] = votes
	}
	return votes[node]
}

/*********************************Protocol***********************************************/
func (corer *DAGBigCore) generatorBlock(view int, height uint8, references []*BlockQC) *Block {
	logger.Debug.Printf("procesing generatorBlock view %d height %d\n", view, height)
	// if view < corer.view || (view == corer.view && height < corer.height) {
	// 	return nil
	// }
	block := &Block{
		Author:     corer.nodeID,
		View:       view,
		Height:     height,
		References: references,
		Batch:      corer.txpool.GetBatch(),
	}
	if block.Batch.ID != -1 {
		//BenchMark Log
		logger.Info.Printf("create Block view %d height %d node %d batch_id %d \n", block.View, block.Height, block.Author, block.Batch.ID)
	}

	return block
}

func (corer *DAGBigCore) handleProposeMsg(p *ProposeMsg) error {
	logger.Debug.Printf("Processing ProposeMsg Proposer %d View %d Height %d\n", p.Author, p.View, p.Height)
	if !p.Verify(corer.committee) {
		return ErrSignature(p.MsgType(), p.View, int(p.Author))
	}
	//Step1: check qc
	// if ok := p.B.Qc.Verify(corer.sigService); ok {

	// }

	//Stpe2: DocG Verify
	if p.DocG != nil && !p.DocG.Verify(corer.committee) {
		return fmt.Errorf("DocG verify error")
	}

	//Step3: store block ? you should check that p.B.qc->B1 store
	if err := storeBlock(corer.store, p.B); err != nil {
		return err
	}

	//AddWeakVotes
	if p.Height == HEIGHT_2 {
		for _, qc := range p.B.References {
			corer.addWeakVotes(qc.View, qc.Proposer)
		}
	}

	go corer.getSpbInstance(p.View, p.Author).processProposeMsg(p)

	return nil
}

func (corer *DAGBigCore) handleVoteMsg(v *VoteMsg) error {
	logger.Debug.Printf("Processing VoteMsg Author %d Proposer %d View %d Height %d\n", v.Author, v.Proposer, v.View, v.Height)
	if !v.Verify(corer.committee) {
		return ErrSignature(v.MsgType(), v.View, int(v.Author))
	}

	go corer.getSpbInstance(v.View, v.Proposer).processVoteMsg(v)

	return nil
}

func (corer *DAGBigCore) handleSpeedVoteMsg(sv *SpeedVoteMsg) error {
	logger.Debug.Printf("Processing SpeedVoteMsg Author %d Proposer %d View %d\n", sv.Author, sv.Proposer, sv.View)
	if !sv.Verify(corer.committee) {
		return ErrSignature(sv.MsgType(), sv.View, int(sv.Author))
	}

	nodes, ok := corer.speedVoteNums[sv.View]
	if !ok {
		nodes = make(map[NodeID]int)
		corer.speedVoteNums[sv.View] = nodes
	}
	nodes[sv.Author]++

	if nodes[corer.nodeID] == corer.committee.HightThreshold() {
		//elect msg
		elect, _ := NewElectMsg(corer.nodeID, sv.View, corer.sigService)
		corer.transmitor.Send(corer.nodeID, NONE, elect)
		corer.transmitor.RecvChannel() <- elect
	}

	go corer.getSpbInstance(sv.View, sv.Proposer).processSpeedVoteMsg(sv)

	return nil
}

func (corer *DAGBigCore) handleElectMsg(e *ElectMsg) error {
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

func (corer *DAGBigCore) processCoin(leader NodeID, view int) error {
	logger.Debug.Printf("Processing CoinLeader Leader %d View %d\n", leader, view)

	spb := corer.getSpbInstance(view, leader)
	if spb.SpeedCert() { //Commit height-2 SpeedVote Commit Rule
		logger.Info.Printf("speed commit Block view %d \n", view)
		blockHash := spb.BlockHash(HEIGHT_2)
		if block, err := getBlock(corer.store, blockHash); err != nil {
			return err
		} else if block != nil {
			corer.commitor.Commit(block)
		}
	} else if corer.getWeakVotes(view, leader) >= corer.committee.LowThreshold() { //Commit f+1 height-2-Block?
		blockHash := spb.BlockHash(HEIGHT_1)
		if block, err := getBlock(corer.store, blockHash); err != nil {
			return err
		} else if block != nil {
			corer.commitor.Commit(block)
		}
	}

	corer.advanceView(view+1, HEIGHT_1)
	// level, qc := spb.QCLEvel()

	// report, _ := NewReportMsg(corer.nodeID, view, level, qc, corer.sigService)
	// corer.transmitor.Send(corer.nodeID, NONE, report)
	// corer.transmitor.RecvChannel() <- report

	return nil
}

func (corer *DAGBigCore) handleReportMsg(r *ReportMsg) error {
	logger.Debug.Printf("Processing ReportMsg Author %d View %d Level %d\n", r.Author, r.View, r.Level)
	if !r.Verify(corer.committee) {
		return ErrSignature(r.MsgType(), r.View, int(r.Author))
	}
	nums := corer.reportNums[r.View]
	nums[r.Level]++
	corer.reportNums[r.View] = nums
	if nums[LEVEL_2] == 1 || nums[LEVEL_1] == corer.committee.HightThreshold() || nums[LEVEL_0] == corer.committee.HightThreshold() {
		corer.advanceView(r.View+1, HEIGHT_1)
	}

	return nil
}

func (corer *DAGBigCore) advanceView(view int, height uint8) {
	//only once for each view&height
	if !((view == corer.view+1 && height == HEIGHT_1) || (view == corer.view && height == corer.height+1)) {
		return
	}
	logger.Debug.Printf("try advance next view %d\n", view)
	var block *Block

	if height == HEIGHT_1 {
		references := corer.getReference(view-1, HEIGHT_2)
		if len(references) >= corer.committee.HightThreshold() {
			block = corer.generatorBlock(view, HEIGHT_1, references)
			if block == nil {
				panic("block is empty")
			}
		}
	} else if height == HEIGHT_2 {
		references := corer.getReference(view, HEIGHT_1)
		if len(references) >= corer.committee.HightThreshold() {
			block = corer.generatorBlock(view, HEIGHT_2, references)
			if block == nil {
				panic("block is empty")
			}
		}
	}

	if block != nil {
		corer.view = view
		corer.height = height
		if propose, err := NewProposeMsg(block, corer.sigService); err != nil {
			logger.Error.Println(err)
			panic(err)
		} else {
			corer.transmitor.Send(corer.nodeID, NONE, propose)
			corer.transmitor.RecvChannel() <- propose
		}
	}
}

func (corer *DAGBigCore) Run() {
	if corer.nodeID >= NodeID(corer.parameters.Faults) {
		//first propose
		logger.Error.Println("DAG-BigCore")
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
		for {
			var err error
			select {
			case msg := <-corer.transmitor.RecvChannel():
				{
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
				}
			case spb := <-corer.callBackSpb:
				{
					corer.advanceView(spb.View, HEIGHT_2)
				}
			}
			if err != nil {
				logger.Warn.Println(err)
			}
		}
	}
}
