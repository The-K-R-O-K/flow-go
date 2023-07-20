package protocol_state

import (
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type ProtocolState struct {
	protocolStateDB storage.ProtocolState
	globalParams    protocol.GlobalParams
}

var _ protocol.ProtocolState = (*ProtocolState)(nil)

func NewProtocolState(protocolStateDB storage.ProtocolState, globalParams protocol.GlobalParams) *ProtocolState {
	return &ProtocolState{
		protocolStateDB: protocolStateDB,
		globalParams:    globalParams,
	}
}

func (s *ProtocolState) AtBlockID(blockID flow.Identifier) (protocol.DynamicProtocolState, error) {
	protocolStateEntry, err := s.protocolStateDB.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not query protocol state at block (%x): %w", blockID, err)
	}
	return newDynamicProtocolStateAdapter(protocolStateEntry, s.globalParams), nil
}

func (s *ProtocolState) GlobalParams() protocol.GlobalParams {
	return s.globalParams
}