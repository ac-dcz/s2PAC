package core

import (
	"bft/2pac/crypto"
	"bft/2pac/logger"
	"bft/2pac/pool"
	"bft/2pac/store"
	"fmt"
)

const (
	HEIGHT_1 uint8 = iota
	HEIGHT_2
)

const (
	LEVEL_2 uint8 = iota //height-2 endorsed QC:
	LEVEL_1              //no_endorsed_height-2_QC, v
	LEVEL_0              //no_endorsed_height-1_QC, v
)

type Core struct {
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
	coinViewFlags map[int]struct{}
	spbInstances  map[int]map[NodeID]*SPB
	finVoteNums   map[int]map[NodeID]struct{}
	reportNums    map[int][3]int
}

func NewCore(
	nodeID NodeID,
	committee Committee,
	parameters Parameters,
	txpool *pool.Pool,
	transmitor *Transmitor,
	store *store.Store,
	sigService *crypto.SigService,
	commitChannel chan<- struct{},
) *Core {

	corer := &Core{
		nodeID:        nodeID,
		committee:     committee,
		view:          0,
		height:        HEIGHT_1,
		parameters:    parameters,
		txpool:        txpool,
		transmitor:    transmitor,
		sigService:    sigService,
		store:         store,
		spbInstances:  make(map[int]map[NodeID]*SPB),
		finVoteNums:   make(map[int]map[NodeID]struct{}),
		reportNums:    make(map[int][3]int),
		coinViewFlags: make(map[int]struct{}),
		eletor:        NewElector(sigService, committee),
		commitor:      NewCommitor(store, commitChannel),
	}

	return corer
}

func storeBlock(store *store.Store, block *Block) error {
	key := block.Hash()
	if val, err := block.Encode(); err != nil {
		return err
	} else {
		store.Write(key[:], val)
		return nil
	}
}

func getBlock(store *store.Store, digest crypto.Digest) (*Block, error) {
	block := &Block{}
	data, err := store.Read(digest[:])
	if err != nil {
		return nil, err
	}
	if err := block.Decode(data); err != nil {
		return nil, err
	}
	return block, nil
}

func (corer *Core) getSpbInstance(view int, node NodeID) *SPB {
	instances, ok := corer.spbInstances[view]
	if !ok {
		instances = make(map[NodeID]*SPB)
		corer.spbInstances[view] = instances
	}
	instance, ok := instances[node]
	if !ok {
		instance = NewSPB(corer, node, view)
		instances[node] = instance
	}
	return instance
}

func (corer *Core) messageFilter(view int) bool {
	return view < corer.view
}

/*********************************Protocol***********************************************/
func (corer *Core) generatorBlock(view int, height uint8, qc *BlockQC) *Block {
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

func (corer *Core) handleProposeMsg(p *ProposeMsg) error {
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

	if corer.messageFilter(p.View) {
		return nil
	}

	go corer.getSpbInstance(p.View, p.Author).processProposeMsg(p)

	return nil
}

func (corer *Core) handleVoteMsg(v *VoteMsg) error {
	logger.Debug.Printf("Processing VoteMsg Author %d Proposer %d View %d Height %d\n", v.Author, v.Proposer, v.View, v.Height)

	if corer.messageFilter(v.View) {
		return nil
	}

	if !v.Verify(corer.committee) {
		return ErrSignature(v.MsgType(), v.View, int(v.Author))
	}

	go corer.getSpbInstance(v.View, v.Proposer).processVoteMsg(v)

	return nil
}

func (corer *Core) handleFinVoteMsg(fv *FinVoteMsg) error {
	logger.Debug.Printf("Processing FinVote Proposer %d View %d", fv.Author, fv.View)

	if corer.messageFilter(fv.View) {
		return nil
	}

	if !fv.Verify(corer.committee) {
		return ErrSignature(fv.MsgType(), fv.View, int(fv.Author))
	}

	finVotes, ok := corer.finVoteNums[fv.View]
	if !ok {
		finVotes = make(map[NodeID]struct{})
		corer.finVoteNums[fv.View] = finVotes
	}
	finVotes[fv.Author] = struct{}{}

	if len(finVotes) == corer.committee.HightThreshold() {
		//elect msg
		elect, _ := NewElectMsg(corer.nodeID, fv.View, corer.sigService)
		corer.transmitor.Send(corer.nodeID, NONE, elect)
		corer.transmitor.RecvChannel() <- elect
	}

	go corer.getSpbInstance(fv.View, fv.Author).processFinVoteMsg(fv)

	return nil
}

func (corer *Core) handleSpeedVoteMsg(sv *SpeedVoteMsg) error {
	logger.Debug.Printf("Processing SpeedVoteMsg Author %d Proposer %d View %d\n", sv.Author, sv.Proposer, sv.View)

	if corer.messageFilter(sv.View) {
		return nil
	}

	if !sv.Verify(corer.committee) {
		return ErrSignature(sv.MsgType(), sv.View, int(sv.Author))
	}

	go corer.getSpbInstance(sv.View, sv.Proposer).processSpeedVoteMsg(sv)

	return nil
}

func (corer *Core) handleElectMsg(e *ElectMsg) error {
	logger.Debug.Printf("Processing ElectMsg Author %d View %d\n", e.Author, e.View)

	if corer.messageFilter(e.View) {
		return nil
	}

	if !e.Verify() {
		return ErrSignature(e.MsgType(), e.View, int(e.Author))
	}

	if leader, err := corer.eletor.Add(e); err != nil {
		return err
	} else if leader != NONE {
		coinMsg := NewRandomCoinMsg(leader, corer.nodeID, e.View, corer.sigService)
		corer.transmitor.Send(corer.nodeID, NONE, coinMsg)
		return corer.processCoin(leader, e.View)
	}

	return nil
}

func (corer *Core) handleCoinMsg(rmsg *RandomCoinMsg) error {
	logger.Debug.Printf("Processing RandomCoinMsg Leader %d Author %d View %d\n", rmsg.Leader, rmsg.Author, rmsg.View)
	if !rmsg.Verify(&corer.committee) {
		return ErrSignature(rmsg.MsgType(), rmsg.View, int(rmsg.Author))
	}
	return corer.processCoin(rmsg.Leader, rmsg.View)
}

func (corer *Core) processCoin(leader NodeID, view int) error {
	logger.Debug.Printf("Processing CoinLeader Leader %d View %d\n", leader, view)

	if _, ok := corer.coinViewFlags[view]; !ok {
		spb := corer.getSpbInstance(view, leader)
		if spb.SpeedCert() {
			logger.Info.Printf("speed commit Block view %d \n", view)
			blockHash := spb.BlockHash(HEIGHT_2)
			if block, err := getBlock(corer.store, blockHash); err != nil {
				return err
			} else if block != nil {
				corer.commitor.Commit(block)
			}
		} else if spb.DecCert() {
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

		corer.coinViewFlags[view] = struct{}{}
	}
	return nil
}

func (corer *Core) handleReportMsg(r *ReportMsg) error {
	logger.Debug.Printf("Processing ReportMsg Author %d View %d Level %d\n", r.Author, r.View, r.Level)

	if corer.messageFilter(r.View) {
		return nil
	}

	if !r.Verify(corer.committee) {
		return ErrSignature(r.MsgType(), r.View, int(r.Author))
	}
	nums := corer.reportNums[r.View]
	nums[r.Level]++
	corer.reportNums[r.View] = nums
	if nums[LEVEL_2] == 1 || nums[LEVEL_1] == corer.committee.HightThreshold() || nums[LEVEL_0] == corer.committee.HightThreshold() {
		docG := NewDocGQCMsg(r.View, r.Level, nil)
		corer.advanceView(r.View+1, r.BlockQc, docG)
	}

	return nil
}

func (corer *Core) advanceView(view int, qc *BlockQC, docG *DocGQCMsg) {
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
		propose.DocG = docG
		corer.transmitor.Send(corer.nodeID, NONE, propose)
		corer.transmitor.RecvChannel() <- propose
	}
}

func (corer *Core) Run() {
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
			case *FinVoteMsg:
				err = corer.handleFinVoteMsg(r)
			case *SpeedVoteMsg:
				err = corer.handleSpeedVoteMsg(r)
			case *ElectMsg:
				err = corer.handleElectMsg(r)
			case *RandomCoinMsg:
				err = corer.handleCoinMsg(r)
			case *ReportMsg:
				err = corer.handleReportMsg(r)
			}

			if err != nil {
				logger.Warn.Println(err)
			}

		}
	}
}
