package core

import (
	"bft/2pac/crypto"
	"bft/2pac/pool"
	"bytes"
	"encoding/gob"
	"reflect"
	"strconv"
)

type ConsensusMessage interface {
	MsgType() int
	Hash() crypto.Digest
}

type Block struct {
	Author     NodeID
	View       int
	Height     uint8
	Batch      pool.Batch
	Qc         *BlockQC
	References []*BlockQC
}

func (b *Block) Encode() ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	if err := gob.NewEncoder(buf).Encode(b); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (b *Block) Decode(data []byte) error {
	buf := bytes.NewBuffer(data)
	if err := gob.NewDecoder(buf).Decode(b); err != nil {
		return err
	}
	return nil
}

func (b *Block) Hash() crypto.Digest {
	hasher := crypto.NewHasher()
	hasher.Add(strconv.AppendInt(nil, int64(b.Author), 2))
	hasher.Add(strconv.AppendInt(nil, int64(b.View), 2))
	hasher.Add(strconv.AppendInt(nil, int64(b.Height), 2))
	hasher.Add(strconv.AppendInt(nil, int64(b.Batch.ID), 2))
	return hasher.Sum256(nil)
}

type ProposeMsg struct {
	Author    NodeID
	B         *Block
	View      int
	Height    uint8
	DocG      *DocGQCMsg
	Signature crypto.Signature
}

func NewProposeMsg(B *Block, sigService *crypto.SigService) (*ProposeMsg, error) {
	p := &ProposeMsg{
		Author: B.Author,
		B:      B,
		View:   B.View,
		Height: B.Height,
	}
	sig, err := sigService.RequestSignature(p.Hash())
	if err != nil {
		return nil, err
	}
	p.Signature = sig
	return p, err
}

func (p *ProposeMsg) Verify(committee Committee) bool {
	pub := committee.Name(p.Author)
	return p.Signature.Verify(pub, p.Hash())
}

func (p *ProposeMsg) Hash() crypto.Digest {
	hasher := crypto.NewHasher()
	hasher.Add(strconv.AppendInt(nil, int64(p.Author), 2))
	hasher.Add(strconv.AppendInt(nil, int64(p.View), 2))
	hasher.Add(strconv.AppendInt(nil, int64(p.Height), 2))
	digest := p.B.Hash()
	hasher.Add(digest[:])
	return hasher.Sum256(nil)
}

func (p *ProposeMsg) MsgType() int {
	return ProposeType
}

type VoteMsg struct {
	Author    NodeID
	Proposer  NodeID
	BlockHash crypto.Digest
	View      int
	Height    uint8
	Signature crypto.Signature
}

func NewVoteMsg(Author NodeID, B *Block, sigService *crypto.SigService) (*VoteMsg, error) {
	v := &VoteMsg{
		Author:    Author,
		Proposer:  B.Author,
		BlockHash: B.Hash(),
		View:      B.View,
		Height:    B.Height,
	}
	sig, err := sigService.RequestSignature(v.Hash())
	if err != nil {
		return nil, err
	}
	v.Signature = sig
	return v, nil
}

func (v *VoteMsg) Verify(committee Committee) bool {
	pub := committee.Name(v.Author)
	return v.Signature.Verify(pub, v.Hash())
}

func (v *VoteMsg) Hash() crypto.Digest {
	hasher := crypto.NewHasher()
	hasher.Add(strconv.AppendInt(nil, int64(v.View), 2))
	hasher.Add(strconv.AppendInt(nil, int64(v.Height), 2))
	hasher.Add(strconv.AppendInt(nil, int64(v.Proposer), 2))
	hasher.Add(v.BlockHash[:])
	return hasher.Sum256(nil)
}

func (v *VoteMsg) MsgType() int {
	return VoteType
}

type FinVoteMsg struct {
	Author    NodeID
	View      int
	Qc        *BlockQC
	Signature crypto.Signature
}

func NewFinVoteMsg(Author NodeID, View int, qc *BlockQC, sigService *crypto.SigService) (*FinVoteMsg, error) {
	f := &FinVoteMsg{
		Author: Author,
		View:   View,
		Qc:     qc,
	}
	sig, err := sigService.RequestSignature(f.Hash())
	if err != nil {
		return nil, err
	}
	f.Signature = sig
	return f, nil
}

func (f *FinVoteMsg) Verify(committee Committee) bool {
	pub := committee.Name(f.Author)
	return f.Signature.Verify(pub, f.Hash())
}

func (f *FinVoteMsg) Hash() crypto.Digest {
	hasher := crypto.NewHasher()
	hasher.Add(strconv.AppendInt(nil, int64(f.Author), 2))
	hasher.Add(strconv.AppendInt(nil, int64(f.View), 2))
	return hasher.Sum256(nil)
}

func (f *FinVoteMsg) MsgType() int {
	return FinVoteType
}

type SpeedVoteMsg struct {
	Author    NodeID
	Proposer  NodeID
	BlockHash crypto.Digest
	View      int
	Signature crypto.Signature
}

func NewSpeedVoteMsg(Author NodeID, qc *BlockQC, sigService *crypto.SigService) (*SpeedVoteMsg, error) {
	v := &SpeedVoteMsg{
		Author:    Author,
		Proposer:  qc.Proposer,
		BlockHash: qc.ParentBlockHash,
		View:      qc.View,
	}
	sig, err := sigService.RequestSignature(v.Hash())
	if err != nil {
		return nil, err
	}
	v.Signature = sig
	return v, nil
}

func (v *SpeedVoteMsg) Verify(committee Committee) bool {
	pub := committee.Name(v.Author)
	return v.Signature.Verify(pub, v.Hash())
}

func (v *SpeedVoteMsg) Hash() crypto.Digest {
	hasher := crypto.NewHasher()
	hasher.Add(strconv.AppendInt(nil, int64(v.View), 2))
	hasher.Add(strconv.AppendInt(nil, int64(v.Proposer), 2))
	hasher.Add(v.BlockHash[:])
	return hasher.Sum256(nil)
}

func (v *SpeedVoteMsg) MsgType() int {
	return SpeedVoteType
}

type BlockQC struct {
	View            int
	Height          uint8
	Proposer        NodeID
	ParentBlockHash crypto.Digest
	Shares          map[NodeID]crypto.SignatureShare
}

func (q *BlockQC) Verify(sigService *crypto.SigService) bool {
	hasher := crypto.NewHasher()
	hasher.Add(strconv.AppendInt(nil, int64(q.View), 2))
	hasher.Add(strconv.AppendInt(nil, int64(q.Height), 2))
	hasher.Add(strconv.AppendInt(nil, int64(q.Proposer), 2))
	hasher.Add(q.ParentBlockHash[:])
	var shares []crypto.SignatureShare
	for _, digest := range q.Shares {
		shares = append(shares, digest)
	}
	_, err := crypto.CombineIntactTSPartial(shares, sigService.ShareKey, hasher.Sum256(nil))
	return err != nil
}

// ElectMsg
type ElectMsg struct {
	Author   NodeID
	View     int
	SigShare crypto.SignatureShare
}

func NewElectMsg(Author NodeID, view int, sigService *crypto.SigService) (*ElectMsg, error) {
	msg := &ElectMsg{
		Author: Author,
		View:   view,
	}
	share, err := sigService.RequestTsSugnature(msg.Hash())
	if err != nil {
		return nil, err
	}
	msg.SigShare = share

	return msg, nil
}

func (msg *ElectMsg) Verify() bool {
	return msg.SigShare.Verify(msg.Hash())
}

func (msg *ElectMsg) Hash() crypto.Digest {
	hasher := crypto.NewHasher()
	hasher.Add(strconv.AppendInt(nil, int64(msg.View), 2))
	return hasher.Sum256(nil)
}

func (msg *ElectMsg) MsgType() int {
	return ElectType
}

type RandomCoinMsg struct {
	Leader    NodeID
	View      int
	Author    NodeID
	Signature crypto.Signature
}

func NewRandomCoinMsg(Leader, Author NodeID, View int, sigService *crypto.SigService) *RandomCoinMsg {
	rmsg := &RandomCoinMsg{
		Leader: Leader,
		View:   View,
		Author: Author,
	}
	sig, _ := sigService.RequestSignature(rmsg.Hash())
	rmsg.Signature = sig
	return rmsg
}

func (msg *RandomCoinMsg) Verify(committee *Committee) bool {
	pub := committee.Name(msg.Author)
	return msg.Signature.Verify(pub, msg.Hash())
}

func (msg *RandomCoinMsg) Hash() crypto.Digest {
	hasher := crypto.NewHasher()
	hasher.Add(strconv.AppendInt(nil, int64(msg.View), 2))
	hasher.Add(strconv.AppendInt(nil, int64(msg.Leader), 2))
	hasher.Add(strconv.AppendInt(nil, int64(msg.Author), 2))
	return hasher.Sum256(nil)
}

func (msg *RandomCoinMsg) MsgType() int {
	return RandomCoinType
}

type ReportMsg struct {
	Author    NodeID
	View      int
	Level     uint8
	BlockQc   *BlockQC
	Signature crypto.Signature
}

func NewReportMsg(Author NodeID, View int, Level uint8, qc *BlockQC, sigService *crypto.SigService) (*ReportMsg, error) {
	r := &ReportMsg{
		Author:  Author,
		View:    View,
		Level:   Level,
		BlockQc: qc,
	}
	sig, err := sigService.RequestSignature(r.Hash())
	if err != nil {
		return nil, err
	}
	r.Signature = sig
	return r, nil
}

func (r *ReportMsg) Verify(committee Committee) bool {
	pub := committee.Name(r.Author)
	return r.Signature.Verify(pub, r.Hash())
}

func (r *ReportMsg) Hash() crypto.Digest {
	hasher := crypto.NewHasher()
	hasher.Add(strconv.AppendInt(nil, int64(r.Author), 2))
	hasher.Add(strconv.AppendInt(nil, int64(r.View), 2))
	hasher.Add(strconv.AppendInt(nil, int64(r.Level), 2))
	return hasher.Sum256(nil)
}

func (r *ReportMsg) MsgType() int {
	return ReportType
}

type DocGQCMsg struct {
	View   int
	Level  uint8
	Shares []*ReportMsg
}

func NewDocGQCMsg(View int, Level uint8, shares []*ReportMsg) *DocGQCMsg {
	m := &DocGQCMsg{
		View:   View,
		Level:  Level,
		Shares: shares,
	}
	return m
}

func (d *DocGQCMsg) Verify(committee Committee) bool {
	for _, share := range d.Shares {
		share.Verify(committee)
	}
	return true
}

func (d *DocGQCMsg) MsgType() int {
	return DocGQCType
}

const (
	ProposeType int = iota
	VoteType
	FinVoteType
	SpeedVoteType
	ElectType
	RandomCoinType
	ReportType
	DocGQCType
)

var DefaultMsgTypes = map[int]reflect.Type{
	ProposeType:    reflect.TypeOf(ProposeMsg{}),
	VoteType:       reflect.TypeOf(VoteMsg{}),
	FinVoteType:    reflect.TypeOf(FinVoteMsg{}),
	SpeedVoteType:  reflect.TypeOf(SpeedVoteMsg{}),
	ElectType:      reflect.TypeOf(ElectMsg{}),
	RandomCoinType: reflect.TypeOf(RandomCoinMsg{}),
	ReportType:     reflect.TypeOf(ReportMsg{}),
	DocGQCType:     reflect.TypeOf(DocGQCMsg{}),
}
