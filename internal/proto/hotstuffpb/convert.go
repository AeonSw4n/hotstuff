package hotstuffpb

import (
	"math/big"

	"github.com/relab/hotstuff"
	"github.com/relab/hotstuff/consensus"
	"github.com/relab/hotstuff/crypto"
	"github.com/relab/hotstuff/crypto/bls12"
	"github.com/relab/hotstuff/crypto/ecdsa"
)

// QuorumSignatureToProto converts a threshold signature to a protocol buffers message.
func QuorumSignatureToProto(sig consensus.QuorumSignature) *QuorumSignature {
	signature := &QuorumSignature{}
	switch s := sig.(type) {
	case ecdsa.MultiSignature:
		sigs := make([]*ECDSASignature, 0, len(s))
		for _, p := range s {
			sigs = append(sigs, &ECDSASignature{
				Signer: uint32(p.Signer()),
				R:      p.R().Bytes(),
				S:      p.S().Bytes(),
			})
		}
		signature.Sig = &QuorumSignature_ECDSASigs{ECDSASigs: &ECDSAMultiSignature{
			Sigs: sigs,
		}}
	case *bls12.AggregateSignature:
		signature.Sig = &QuorumSignature_BLS12Sig{BLS12Sig: &BLS12AggregateSignature{
			Sig:          s.ToBytes(),
			Participants: s.Bitfield().Bytes(),
		}}
	}
	return signature
}

// QuorumSignatureFromProto converts a protocol buffers message to a threshold signature.
func QuorumSignatureFromProto(sig *QuorumSignature) consensus.QuorumSignature {
	if signature := sig.GetECDSASigs(); signature != nil {
		sigs := make([]*ecdsa.Signature, len(signature.GetSigs()))
		for i, sig := range signature.GetSigs() {
			r := new(big.Int)
			r.SetBytes(sig.GetR())
			s := new(big.Int)
			s.SetBytes(sig.GetS())
			sigs[i] = ecdsa.RestoreSignature(r, s, hotstuff.ID(sig.GetSigner()))
		}
		return ecdsa.RestoreMultiSignature(sigs)
	}
	if signature := sig.GetBLS12Sig(); signature != nil {
		aggSig, err := bls12.RestoreAggregateSignature(signature.GetSig(), crypto.BitfieldFromBytes(signature.GetParticipants()))
		if err != nil {
			return nil
		}
		return aggSig
	}
	return nil
}

// PartialCertToProto converts a consensus.PartialCert to a hotstuffpb.Partialcert.
func PartialCertToProto(cert consensus.PartialCert) *PartialCert {
	hash := cert.BlockHash()
	return &PartialCert{
		Sig:  QuorumSignatureToProto(cert.Signature()),
		Hash: hash[:],
	}
}

// PartialCertFromProto converts a hotstuffpb.PartialCert to an ecdsa.PartialCert.
func PartialCertFromProto(cert *PartialCert) consensus.PartialCert {
	var h consensus.Hash
	copy(h[:], cert.GetHash())
	return consensus.NewPartialCert(QuorumSignatureFromProto(cert.GetSig()), h)
}

// QuorumCertToProto converts a consensus.QuorumCert to a hotstuffpb.QuorumCert.
func QuorumCertToProto(qc consensus.QuorumCert) *QuorumCert {
	hash := qc.BlockHash()
	return &QuorumCert{
		Sig:  QuorumSignatureToProto(qc.Signature()),
		Hash: hash[:],
		View: uint64(qc.View()),
	}
}

// QuorumCertFromProto converts a hotstuffpb.QuorumCert to an ecdsa.QuorumCert.
func QuorumCertFromProto(qc *QuorumCert) consensus.QuorumCert {
	var h consensus.Hash
	copy(h[:], qc.GetHash())
	return consensus.NewQuorumCert(QuorumSignatureFromProto(qc.GetSig()), consensus.View(qc.GetView()), h)
}

// ProposalToProto converts a ProposeMsg to a protobuf message.
func ProposalToProto(proposal consensus.ProposeMsg) *Proposal {
	p := &Proposal{
		Block: BlockToProto(proposal.Block),
	}
	if proposal.AggregateQC != nil {
		p.AggQC = AggregateQCToProto(*proposal.AggregateQC)
	}
	return p
}

// ProposalFromProto converts a protobuf message to a ProposeMsg.
func ProposalFromProto(p *Proposal) (proposal consensus.ProposeMsg) {
	proposal.Block = BlockFromProto(p.GetBlock())
	if p.GetAggQC() != nil {
		aggQC := AggregateQCFromProto(p.GetAggQC())
		proposal.AggregateQC = &aggQC
	}
	return
}

// BlockToProto converts a consensus.Block to a hotstuffpb.Block.
func BlockToProto(block *consensus.Block) *Block {
	parentHash := block.Parent()
	return &Block{
		Parent:   parentHash[:],
		Command:  []byte(block.Command()),
		QC:       QuorumCertToProto(block.QuorumCert()),
		View:     uint64(block.View()),
		Proposer: uint32(block.Proposer()),
	}
}

// BlockFromProto converts a hotstuffpb.Block to a consensus.Block.
func BlockFromProto(block *Block) *consensus.Block {
	var p consensus.Hash
	copy(p[:], block.GetParent())
	return consensus.NewBlock(
		p,
		QuorumCertFromProto(block.GetQC()),
		consensus.Command(block.GetCommand()),
		consensus.View(block.GetView()),
		hotstuff.ID(block.GetProposer()),
	)
}

// TimeoutMsgFromProto converts a TimeoutMsg proto to the hotstuff type.
func TimeoutMsgFromProto(m *TimeoutMsg) consensus.TimeoutMsg {
	timeoutMsg := consensus.TimeoutMsg{
		View:          consensus.View(m.GetView()),
		SyncInfo:      SyncInfoFromProto(m.GetSyncInfo()),
		ViewSignature: QuorumSignatureFromProto(m.GetViewSig()),
	}
	if m.GetViewSig() != nil {
		timeoutMsg.MsgSignature = QuorumSignatureFromProto(m.GetMsgSig())
	}
	return timeoutMsg
}

// TimeoutMsgToProto converts a TimeoutMsg to the protobuf type.
func TimeoutMsgToProto(timeoutMsg consensus.TimeoutMsg) *TimeoutMsg {
	tm := &TimeoutMsg{
		View:     uint64(timeoutMsg.View),
		SyncInfo: SyncInfoToProto(timeoutMsg.SyncInfo),
		ViewSig:  QuorumSignatureToProto(timeoutMsg.ViewSignature),
	}
	if timeoutMsg.MsgSignature != nil {
		tm.MsgSig = QuorumSignatureToProto(timeoutMsg.MsgSignature)
	}
	return tm
}

// TimeoutCertFromProto converts a timeout certificate from the protobuf type to the hotstuff type.
func TimeoutCertFromProto(m *TimeoutCert) consensus.TimeoutCert {
	return consensus.NewTimeoutCert(QuorumSignatureFromProto(m.GetSig()), consensus.View(m.GetView()))
}

// TimeoutCertToProto converts a timeout certificate from the hotstuff type to the protobuf type.
func TimeoutCertToProto(timeoutCert consensus.TimeoutCert) *TimeoutCert {
	return &TimeoutCert{
		View: uint64(timeoutCert.View()),
		Sig:  QuorumSignatureToProto(timeoutCert.Signature()),
	}
}

// AggregateQCFromProto converts an AggregateQC from the protobuf type to the hotstuff type.
func AggregateQCFromProto(m *AggQC) consensus.AggregateQC {
	qcs := make(map[hotstuff.ID]consensus.QuorumCert)
	for id, pQC := range m.GetQCs() {
		qcs[hotstuff.ID(id)] = QuorumCertFromProto(pQC)
	}
	return consensus.NewAggregateQC(qcs, QuorumSignatureFromProto(m.GetSig()), consensus.View(m.GetView()))
}

// AggregateQCToProto converts an AggregateQC from the hotstuff type to the protobuf type.
func AggregateQCToProto(aggQC consensus.AggregateQC) *AggQC {
	pQCs := make(map[uint32]*QuorumCert, len(aggQC.QCs()))
	for id, qc := range aggQC.QCs() {
		pQCs[uint32(id)] = QuorumCertToProto(qc)
	}
	return &AggQC{QCs: pQCs, Sig: QuorumSignatureToProto(aggQC.Sig()), View: uint64(aggQC.View())}
}

// SyncInfoFromProto converts a SyncInfo struct from the protobuf type to the hotstuff type.
func SyncInfoFromProto(m *SyncInfo) consensus.SyncInfo {
	si := consensus.NewSyncInfo()
	if qc := m.GetQC(); qc != nil {
		si = si.WithQC(QuorumCertFromProto(qc))
	}
	if tc := m.GetTC(); tc != nil {
		si = si.WithTC(TimeoutCertFromProto(tc))
	}
	if aggQC := m.GetAggQC(); aggQC != nil {
		si = si.WithAggQC(AggregateQCFromProto(aggQC))
	}
	return si
}

// SyncInfoToProto converts a SyncInfo struct from the hotstuff type to the protobuf type.
func SyncInfoToProto(syncInfo consensus.SyncInfo) *SyncInfo {
	m := &SyncInfo{}
	if qc, ok := syncInfo.QC(); ok {
		m.QC = QuorumCertToProto(qc)
	}
	if tc, ok := syncInfo.TC(); ok {
		m.TC = TimeoutCertToProto(tc)
	}
	if aggQC, ok := syncInfo.AggQC(); ok {
		m.AggQC = AggregateQCToProto(aggQC)
	}
	return m
}
