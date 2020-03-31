// Code generated by mockery v1.0.0. DO NOT EDIT.

package mock

import dkg "github.com/dapperlabs/flow-go/state/dkg"
import flow "github.com/dapperlabs/flow-go/model/flow"

import mock "github.com/stretchr/testify/mock"

// ViewState is an autogenerated mock type for the ViewState type
type ViewState struct {
	mock.Mock
}

// AllConsensusParticipants provides a mock function with given fields: blockID
func (_m *ViewState) AllConsensusParticipants(blockID flow.Identifier) (flow.IdentityList, error) {
	ret := _m.Called(blockID)

	var r0 flow.IdentityList
	if rf, ok := ret.Get(0).(func(flow.Identifier) flow.IdentityList); ok {
		r0 = rf(blockID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(flow.IdentityList)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(flow.Identifier) error); ok {
		r1 = rf(blockID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// DKGState provides a mock function with given fields:
func (_m *ViewState) DKGState() dkg.State {
	ret := _m.Called()

	var r0 dkg.State
	if rf, ok := ret.Get(0).(func() dkg.State); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(dkg.State)
		}
	}

	return r0
}

// IdentitiesForConsensusParticipants provides a mock function with given fields: blockID, consensusNodeIDs
func (_m *ViewState) IdentitiesForConsensusParticipants(blockID flow.Identifier, consensusNodeIDs ...flow.Identifier) (flow.IdentityList, error) {
	_va := make([]interface{}, len(consensusNodeIDs))
	for _i := range consensusNodeIDs {
		_va[_i] = consensusNodeIDs[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, blockID)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 flow.IdentityList
	if rf, ok := ret.Get(0).(func(flow.Identifier, ...flow.Identifier) flow.IdentityList); ok {
		r0 = rf(blockID, consensusNodeIDs...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(flow.IdentityList)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(flow.Identifier, ...flow.Identifier) error); ok {
		r1 = rf(blockID, consensusNodeIDs...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// IdentityForConsensusParticipant provides a mock function with given fields: blockID, participantID
func (_m *ViewState) IdentityForConsensusParticipant(blockID flow.Identifier, participantID flow.Identifier) (*flow.Identity, error) {
	ret := _m.Called(blockID, participantID)

	var r0 *flow.Identity
	if rf, ok := ret.Get(0).(func(flow.Identifier, flow.Identifier) *flow.Identity); ok {
		r0 = rf(blockID, participantID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.Identity)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(flow.Identifier, flow.Identifier) error); ok {
		r1 = rf(blockID, participantID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// IsSelf provides a mock function with given fields: nodeID
func (_m *ViewState) IsSelf(nodeID flow.Identifier) bool {
	ret := _m.Called(nodeID)

	var r0 bool
	if rf, ok := ret.Get(0).(func(flow.Identifier) bool); ok {
		r0 = rf(nodeID)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// IsSelfLeaderForView provides a mock function with given fields: view
func (_m *ViewState) IsSelfLeaderForView(view uint64) bool {
	ret := _m.Called(view)

	var r0 bool
	if rf, ok := ret.Get(0).(func(uint64) bool); ok {
		r0 = rf(view)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// LeaderForView provides a mock function with given fields: view
func (_m *ViewState) LeaderForView(view uint64) *flow.Identity {
	ret := _m.Called(view)

	var r0 *flow.Identity
	if rf, ok := ret.Get(0).(func(uint64) *flow.Identity); ok {
		r0 = rf(view)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.Identity)
		}
	}

	return r0
}

// Self provides a mock function with given fields:
func (_m *ViewState) Self() flow.Identifier {
	ret := _m.Called()

	var r0 flow.Identifier
	if rf, ok := ret.Get(0).(func() flow.Identifier); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(flow.Identifier)
		}
	}

	return r0
}
