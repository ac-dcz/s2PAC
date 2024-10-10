package core

import (
	"bft/2pac/crypto"
	"bft/2pac/logger"
	"bft/2pac/store"
)

type Commitor struct {
	lastView        int
	lastHeight      uint8
	committedBlcoks map[crypto.Digest]struct{}
	blocksChan      chan *Block
	store           *store.Store
	commitChannel   chan<- struct{}
}

func NewCommitor(store *store.Store, commitBack chan<- struct{}) *Commitor {
	c := &Commitor{
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

func (c *Commitor) run() {
	for block := range c.blocksChan {
		if _, ok := c.committedBlcoks[block.Hash()]; !ok {
			queue := []*Block{block}
			current := block
			for current != nil && current.Qc != nil {
				if b, err := getBlock(c.store, current.Qc.ParentBlockHash); b != nil && err == nil {
					if _, ok = c.committedBlcoks[b.Hash()]; ok {
						break
					}
					queue = append(queue, b)
					current = b
				}
			}

			for i := len(queue) - 1; i >= 0; i-- {
				block := queue[i]
				if block.Batch.ID != -1 {
					logger.Info.Printf("commit Block view %d height %d node %d batch_id %d \n", block.View, block.Height, block.Author, block.Batch.ID)
				}
				c.committedBlcoks[block.Hash()] = struct{}{}
				c.commitChannel <- struct{}{}
			}

		}
	}
}

func (c *Commitor) Commit(block *Block) {
	if block.View < c.lastView || (block.View == c.lastView && block.Height < c.lastHeight) {
		return
	}
	c.lastView, c.lastHeight = block.View, block.Height
	c.blocksChan <- block
}
