package protocol_test

import (
	"github.com/onflow/flow-go/state/protocol"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/utils/unittest"
)

var participants = unittest.IdentityListFixture(20, unittest.WithAllRoles())

func TestEpochSetupValidity(t *testing.T) {
	t.Run("invalid first/final view", func(t *testing.T) {
		_, result, _ := unittest.BootstrapFixture(participants)
		setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		// set an invalid final view for the first epoch
		setup.FinalView = setup.FirstView

		err := protocol.VerifyEpochSetup(setup, true)
		require.Error(t, err)
	})

	t.Run("non-canonically ordered identities", func(t *testing.T) {
		_, result, _ := unittest.BootstrapFixture(participants)
		setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		// randomly shuffle the identities so they are not canonically ordered
		var err error
		setup.Participants, err = setup.Participants.Shuffle()
		require.NoError(t, err)
		err = protocol.VerifyEpochSetup(setup, true)
		require.Error(t, err)
	})

	t.Run("invalid cluster assignments", func(t *testing.T) {
		_, result, _ := unittest.BootstrapFixture(participants)
		setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		// create an invalid cluster assignment (node appears in multiple clusters)
		collector := participants.Filter(filter.HasRole(flow.RoleCollection))[0]
		setup.Assignments = append(setup.Assignments, []flow.Identifier{collector.NodeID})

		err := protocol.VerifyEpochSetup(setup, true)
		require.Error(t, err)
	})

	t.Run("short seed", func(t *testing.T) {
		_, result, _ := unittest.BootstrapFixture(participants)
		setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		setup.RandomSource = unittest.SeedFixture(crypto.SeedMinLenDKG - 1)

		err := protocol.VerifyEpochSetup(setup, true)
		require.Error(t, err)
	})
}

func TestBootstrapInvalidEpochCommit(t *testing.T) {
	t.Run("inconsistent counter", func(t *testing.T) {
		_, result, _ := unittest.BootstrapFixture(participants)
		setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		commit := result.ServiceEvents[1].Event.(*flow.EpochCommit)
		// use a different counter for the commit
		commit.Counter = setup.Counter + 1

		err := protocol.IsValidEpochCommit(commit, setup)
		require.Error(t, err)
	})

	t.Run("inconsistent cluster QCs", func(t *testing.T) {
		_, result, _ := unittest.BootstrapFixture(participants)
		setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		commit := result.ServiceEvents[1].Event.(*flow.EpochCommit)
		// add an extra QC to commit
		extraQC := unittest.QuorumCertificateWithSignerIDsFixture()
		commit.ClusterQCs = append(commit.ClusterQCs, flow.ClusterQCVoteDataFromQC(extraQC))

		err := protocol.IsValidEpochCommit(commit, setup)
		require.Error(t, err)
	})

	t.Run("missing dkg group key", func(t *testing.T) {
		_, result, _ := unittest.BootstrapFixture(participants)
		setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		commit := result.ServiceEvents[1].Event.(*flow.EpochCommit)
		commit.DKGGroupKey = nil

		err := protocol.IsValidEpochCommit(commit, setup)
		require.Error(t, err)
	})

	t.Run("inconsistent DKG participants", func(t *testing.T) {
		_, result, _ := unittest.BootstrapFixture(participants)
		setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
		commit := result.ServiceEvents[1].Event.(*flow.EpochCommit)
		// add an extra DKG participant key
		commit.DKGParticipantKeys = append(commit.DKGParticipantKeys, unittest.KeyFixture(crypto.BLSBLS12381).PublicKey())

		err := protocol.IsValidEpochCommit(commit, setup)
		require.Error(t, err)
	})
}