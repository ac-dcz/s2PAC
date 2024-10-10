package core

import "fmt"

var (
	ErrSignature = func(msgTyp, view, node int) error {
		return fmt.Errorf("[type-%d-view-%d-node-%d] message signature verify error", msgTyp, view, node)
	}

	ErrReference = func(msgTyp, view, node int) error {
		return fmt.Errorf("[type-%d-view-%d-node-%d] not receive all block reference ", msgTyp, view, node)
	}

	ErrUsedElect = func(msgTyp, view, node int) error {
		return fmt.Errorf("[type-%d-view-%d-node-%d] receive one more elect msg from %d ", msgTyp, view, node, node)
	}
)
