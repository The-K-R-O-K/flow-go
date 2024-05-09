package inmem

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
)

// EncodableSnapshot is the encoding format for protocol.Snapshot
type EncodableSnapshot struct {
	LatestSeal          *flow.Seal            // TODO replace with same info from sealing segment
	LatestResult        *flow.ExecutionResult // TODO replace with same info from sealing segment
	SealingSegment      *flow.SealingSegment
	QuorumCertificate   *flow.QuorumCertificate
	Params              EncodableParams
	SealedVersionBeacon *flow.SealedVersionBeacon
}

// Head returns the latest finalized header of the Snapshot, which is the block
// in the sealing segment with the greatest Height.
// The EncodableSnapshot receiver must be correctly formed.
func (snap EncodableSnapshot) Head() *flow.Header {
	return snap.SealingSegment.Highest().Header
}

// GetLatestSeal returns the latest seal of the Snapshot. This is the seal
// for the block with the greatest height, of all seals in the Snapshot.
// The EncodableSnapshot receiver must be correctly formed.
func (snap EncodableSnapshot) GetLatestSeal() *flow.Seal {
	// CASE 1: In the case of a spork root, the single block of the sealing
	// segment is sealed by protocol definition by `FirstSeal`.
	if snap.SealingSegment.IsSporkRoot() {
		return snap.SealingSegment.FirstSeal
	}

	head := snap.Head()
	latestSealID := snap.SealingSegment.LatestSeals[head.ID()]

	// CASE 2: For a mid-spork root snapshot, there are multiple blocks in the sealing segment.
	// Since seals are included in increasing height order, the latest seal must be in the
	// first block (by height descending) which contains any seals.
	for i := len(snap.SealingSegment.Blocks) - 1; i >= 0; i-- {
		block := snap.SealingSegment.Blocks[i]
		for _, seal := range block.Payload.Seals {
			if seal.ID() == latestSealID {
				return seal
			}
		}
		if len(block.Payload.Seals) > 0 {
			// We encountered a block with some seals, but not the latest seal.
			// This can only occur in a structurally invalid SealingSegment.
			panic("sanity check failed: no latest seal")
		}
	}
	// Correctly formatted sealing segments must contain latest seal.
	panic("unreachable for correctly formatted sealing segments")
}

// GetLatestResult returns the latest sealed result of the Snapshot.
// This is the result which is sealed by LatestSeal.
// The EncodableSnapshot receiver must be correctly formed.
func (snap EncodableSnapshot) GetLatestResult() *flow.ExecutionResult {
	latestSeal := snap.GetLatestSeal()

	for _, result := range snap.SealingSegment.ExecutionResults {
		if latestSeal.ResultID == result.ID() {
			return result
		}
	}
	for _, block := range snap.SealingSegment.Blocks {
		for _, result := range block.Payload.Results {
			if latestSeal.ResultID == result.ID() {
				return result
			}
		}
	}
	// Correctly formatted sealing segments must contain latest result.
	panic("unreachable for correctly formatted sealing segments")
}

// EncodableDKG is the encoding format for protocol.DKG
type EncodableDKG struct {
	GroupKey     encodable.RandomBeaconPubKey
	Participants map[flow.Identifier]flow.DKGParticipant
}

type EncodableFullDKG struct {
	GroupKey      encodable.RandomBeaconPubKey
	PrivKeyShares []encodable.RandomBeaconPrivKey
	PubKeyShares  []encodable.RandomBeaconPubKey
}

// EncodableCluster is the encoding format for protocol.Cluster
type EncodableCluster struct {
	Index     uint
	Counter   uint64
	Members   flow.IdentitySkeletonList
	RootBlock *cluster.Block
	RootQC    *flow.QuorumCertificate
}

// EncodableParams is the encoding format for protocol.GlobalParams
type EncodableParams struct {
	ChainID                    flow.ChainID
	SporkID                    flow.Identifier
	SporkRootBlockHeight       uint64
	ProtocolVersion            uint
	EpochCommitSafetyThreshold uint64
}
