package flow

// DynamicIdentityEntry encapsulates nodeID and dynamic portion of identity.
type DynamicIdentityEntry struct {
	NodeID  Identifier
	Dynamic DynamicIdentity
}

type DynamicIdentityEntryList []*DynamicIdentityEntry

// ProtocolStateEntry holds information about the protocol state at some point in time.
// It allows to reconstruct the state of identity table using epoch setup events and dynamic identities.
// It tracks attempts of invalid state transitions.
// It also holds information about the next epoch, if it has been already committed.
// This structure is used to persist protocol state in the database.
type ProtocolStateEntry struct {
	// setup and commit event IDs for current epoch.
	CurrentEpochEventIDs EventIDs
	// setup and commit event IDs for previous epoch.
	PreviousEpochEventIDs EventIDs
	// Part of identity table that can be changed during the epoch.
	Identities DynamicIdentityEntryList
	// InvalidStateTransitionAttempted encodes whether an invalid state transition
	// has been detected in this fork. When this happens, epoch fallback is triggered
	// AFTER the fork is finalized.
	InvalidStateTransitionAttempted bool
	// NextEpochProtocolState describes protocol state of the next epoch
	NextEpochProtocolState *ProtocolStateEntry
}

// RichProtocolStateEntry is a ProtocolStateEntry which has additional fields that are cached
// from storage layer for convenience.
// Using this structure instead of ProtocolStateEntry allows us to avoid querying
// the database for epoch setups and commits and full identity table.
// It holds several invariants, such as:
// - CurrentEpochSetup and CurrentEpochCommit are for the same epoch. Never nil.
// - PreviousEpochSetup and PreviousEpochCommit are for the same epoch. Never nil.
// - Identities is a full identity table for the current epoch. Identities are sorted in canonical order. Never nil.
// - NextEpochProtocolState is a protocol state for the next epoch. Can be nil.
type RichProtocolStateEntry struct {
	ProtocolStateEntry

	CurrentEpochSetup   *EpochSetup
	CurrentEpochCommit  *EpochCommit
	PreviousEpochSetup  *EpochSetup
	PreviousEpochCommit *EpochCommit
	Identities          IdentityList

	NextEpochProtocolState *RichProtocolStateEntry
}

// ID returns hash of entry by hashing all fields.
func (e *ProtocolStateEntry) ID() Identifier {
	if e == nil {
		return ZeroID
	}
	body := struct {
		CurrentEpochEventIDs            Identifier
		PreviousEpochEventIDs           Identifier
		Identities                      DynamicIdentityEntryList
		InvalidStateTransitionAttempted bool
		NextEpochProtocolStateID        Identifier
	}{
		CurrentEpochEventIDs:            e.CurrentEpochEventIDs.ID(),
		PreviousEpochEventIDs:           e.PreviousEpochEventIDs.ID(),
		Identities:                      e.Identities,
		InvalidStateTransitionAttempted: e.InvalidStateTransitionAttempted,
		NextEpochProtocolStateID:        e.NextEpochProtocolState.ID(),
	}
	return MakeID(body)
}