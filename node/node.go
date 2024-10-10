package node

import (
	"bft/2pac/config"
	"bft/2pac/core"
	"bft/2pac/crypto"
	"bft/2pac/logger"
	"bft/2pac/pool"
	"bft/2pac/store"
	"fmt"
)

type Node struct {
	commitChannel chan struct{}
}

func NewNode(
	keysFile, tssKeyFile, committeeFile, parametersFile, storePath, logPath string,
	logLevel, nodeID int,
) (*Node, error) {

	commitChannel := make(chan struct{}, 1_000)
	//step 1: init log config
	logger.SetOutput(logger.InfoLevel, logger.NewFileWriter(fmt.Sprintf("%s/node-info-%d.log", logPath, nodeID)))
	logger.SetOutput(logger.DebugLevel, logger.NewFileWriter(fmt.Sprintf("%s/node-debug-%d.log", logPath, nodeID)))
	logger.SetOutput(logger.WarnLevel, logger.NewFileWriter(fmt.Sprintf("%s/node-warn-%d.log", logPath, nodeID)))
	logger.SetOutput(logger.ErrorLevel, logger.NewFileWriter(fmt.Sprintf("%s/node-error-%d.log", logPath, nodeID)))
	logger.SetLevel(logger.Level(logLevel))

	//step 2: ReadKeys
	_, priKey, err := config.GenKeysFromFile(keysFile)
	if err != nil {
		logger.Error.Println(err)
		return nil, err
	}

	shareKey, err := config.GenTsKeyFromFile(tssKeyFile)
	if err != nil {
		logger.Error.Println(err)
		return nil, err
	}

	//step 3: committee and parameters
	commitee, err := config.GenCommitteeFromFile(committeeFile)
	if err != nil {
		logger.Error.Println(err)
		return nil, err
	}

	poolParameters, coreParameters, err := config.GenParamatersFromFile(parametersFile)
	if err != nil {
		logger.Error.Println(err)
		return nil, err
	}

	//step 4: invoke pool and core
	txpool := pool.NewPool(poolParameters, commitee.Size(), nodeID)

	_store := store.NewStore(store.NewDefaultNutsDB(storePath))
	sigService := crypto.NewSigService(priKey, shareKey)

	if err = core.Consensus(
		core.NodeID(nodeID),
		commitee,
		coreParameters,
		txpool,
		_store,
		sigService,
		commitChannel,
	); err != nil {
		logger.Error.Println(err)
		return nil, err
	}

	// txpool.Run()
	logger.Info.Printf("Node %d successfully booted \n", nodeID)

	return &Node{
		commitChannel: commitChannel,
	}, nil
}

// AnalyzeBlock: block
func (n *Node) AnalyzeBlock() {
	for range n.commitChannel {
		//to do something
	}
}
