package core

import (
	"bft/2pac/crypto"
	"bft/2pac/logger"
	"bft/2pac/store"
)

type DAGCommitor struct {
	lastView        int
	lastHeight      uint8
	committedBlcoks map[crypto.Digest]struct{}
	blocksChan      chan *Block
	store           *store.Store
	commitChannel   chan<- struct{}
}

func NewDAGCommitor(store *store.Store, commitBack chan<- struct{}) *DAGCommitor {
	c := &DAGCommitor{
		lastView:        0,
		lastHeight:      HEIGHT_1,
		committedBlcoks: make(map[crypto.Digest]struct{}),
		blocksChan:      make(chan *Block, 1_00),
		store:           store,
		commitChannel:   commitBack,
	}
	go c.run()
	return c
}

func (c *DAGCommitor) run() {
	for block := range c.blocksChan {

		queue := []*Block{block}
		for len(queue) > 0 {
			block := queue[0]
			digest := block.Hash()
			if _, ok := c.committedBlcoks[digest]; !ok {
				if block.Batch.ID != -1 {
					logger.Info.Printf("commit Block view %d height %d node %d batch_id %d \n", block.View, block.Height, block.Author, block.Batch.ID)
				}
				c.committedBlcoks[digest] = struct{}{}
				c.commitChannel <- struct{}{}
				for _, qc := range block.References {
					if b, err := getBlock(c.store, qc.ParentBlockHash); b != nil && err == nil {
						queue = append(queue, b)
					}
				}
			}
			queue = queue[1:]
		}
	}
}

func (c *DAGCommitor) Commit(block *Block) {
	if block.View < c.lastView || (block.View == c.lastView && block.Height < c.lastHeight) {
		return
	}
	c.lastView, c.lastHeight = block.View, block.Height
	c.blocksChan <- block
}
