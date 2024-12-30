package core

import (
	"bft/2pac/crypto"
	"bft/2pac/logger"
	"bft/2pac/network"
	"bft/2pac/pool"
	"bft/2pac/store"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	PAC_LEAN int = iota
	PAC_BIG
	PAC_BIG_DAG
)

func Consensus(
	id NodeID,
	committee Committee,
	parameters Parameters,
	txpool *pool.Pool,
	store *store.Store,
	sigService *crypto.SigService,
	commitChannel chan<- struct{},
) error {
	logger.Info.Printf(
		"Consensus Node ID: %d\n",
		id,
	)
	// logger.Info.Printf(
	// 	"Consensus committee: %+v\n",
	// 	committee,
	// )
	logger.Info.Printf(
		"Consensus DDos: %v, Faults: %v \n",
		parameters.DDos, parameters.Faults,
	)
	if id < NodeID(parameters.Faults) {
		logger.Info.Println("Byzantine Node")
	} else {
		logger.Info.Println("Honest Node")
	}

	//Step 1: invoke network
	cc := network.NewCodec(DefaultMsgTypes)
	addr := fmt.Sprintf(":%s", strings.Split(committee.Address(id), ":")[1])
	sender, receiver := network.NewSender(cc), network.NewReceiver(addr, cc)
	go sender.Run()
	go receiver.Run()
	transmitor := NewTransmitor(sender, receiver, parameters, committee)

	//Step 2: Waiting for all nodes to be online
	logger.Info.Println("Waiting for all nodes to be online...")
	wg := sync.WaitGroup{}
	addrs := committee.BroadCast(id)
	for _, addr := range addrs {
		wg.Add(1)
		go func(address string) {
			defer wg.Done()
			for {
				conn, err := net.Dial("tcp", address)
				if err != nil {
					time.Sleep(time.Millisecond * 10)
					continue
				}
				conn.Close()
				return
			}
		}(addr)
	}
	wg.Wait()
	logger.Error.Println("test-1")
	time.Sleep(time.Millisecond * time.Duration(parameters.SyncTimeout))
	txpool.Run()
	logger.Error.Println("test-2")
	//Step 3: start protocol
	if parameters.Protocol == PAC_LEAN {
		corer := NewCore(id, committee, parameters, txpool, transmitor, store, sigService, commitChannel)
		go corer.Run()
	} else if parameters.Protocol == PAC_BIG {
		corer := NewBigCore(id, committee, parameters, txpool, transmitor, store, sigService, commitChannel)
		go corer.Run()
	} else if parameters.Protocol == PAC_BIG_DAG {
		corer := NewDAGBigCore(id, committee, parameters, txpool, transmitor, store, sigService, commitChannel)
		go corer.Run()
	} else {
		panic("the type of protocol is invaild")
	}

	return nil
}
