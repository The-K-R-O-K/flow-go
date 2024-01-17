package scoring_test

import (
	"context"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/p2p"
	netcache "github.com/onflow/flow-go/network/p2p/cache"
	p2pconfig "github.com/onflow/flow-go/network/p2p/config"
	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
	mockp2p "github.com/onflow/flow-go/network/p2p/mock"
	"github.com/onflow/flow-go/network/p2p/scoring"
	"github.com/onflow/flow-go/network/p2p/scoring/internal"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestScoreRegistry_FreshStart tests the app specific score computation of the node when there is no spam record for the peer id upon fresh start of the registry.
// It tests the state that a staked peer with a valid role and valid subscriptions has no spam records; hence it should "eventually" be rewarded with the default reward
// for its GossipSub app specific score. The "eventually" comes from the fact that the app specific score is updated asynchronously in the cache, and the cache is
// updated when the app specific score function is called by GossipSub.
func TestScoreRegistry_FreshStart(t *testing.T) {
	peerID := peer.ID("peer-1")

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)
	// refresh cached app-specific score every 100 milliseconds to speed up the test.
	cfg.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters.AppSpecificScore.ScoreTTL = 100 * time.Millisecond

	reg, spamRecords, appScoreCache := newGossipSubAppSpecificScoreRegistry(t,
		cfg.NetworkConfig.GossipSub.ScoringParameters,
		scoring.InitAppScoreRecordStateFunc(cfg.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters.SpamRecordCache.Decay.MaximumSpamPenaltyDecayFactor),
		withStakedIdentities(peerID),
		withValidSubscriptions(peerID))

	// starts the registry.
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	reg.Start(signalerCtx)
	unittest.RequireCloseBefore(t, reg.Ready(), 1*time.Second, "failed to start GossipSubAppSpecificScoreRegistry")

	// initially, the spamRecords should not have the peer id, and there should be no app-specific score in the cache.
	require.False(t, spamRecords.Has(peerID))
	score, updated, exists := appScoreCache.Get(peerID) // get the score from the cache.
	require.False(t, exists)
	require.Equal(t, time.Time{}, updated)
	require.Equal(t, float64(0), score)

	maxAppSpecificReward := cfg.NetworkConfig.GossipSub.ScoringParameters.PeerScoring.Protocol.AppSpecificScore.MaxAppSpecificReward

	queryTime := time.Now()
	require.Eventually(t, func() bool {
		// calling the app specific score function when there is no app specific score in the cache should eventually update the cache.
		score := reg.AppSpecificScoreFunc()(peerID)
		// since the peer id does not have a spam record, the app specific score should be the max app specific reward, which
		// is the default reward for a staked peer that has valid subscriptions.
		return score == maxAppSpecificReward
	}, 5*time.Second, 100*time.Millisecond)

	// still the spamRecords should not have the peer id (as there is no spam record for the peer id).
	require.False(t, spamRecords.Has(peerID))

	// however, the app specific score should be updated in the cache.
	score, updated, exists = appScoreCache.Get(peerID) // get the score from the cache.
	require.True(t, exists)
	require.True(t, updated.After(queryTime))
	require.Equal(t, maxAppSpecificReward, score)

	// stop the registry.
	cancel()
	unittest.RequireCloseBefore(t, reg.Done(), 1*time.Second, "failed to stop GossipSubAppSpecificScoreRegistry")
}

// TestScoreRegistry_PeerWithSpamRecord is a test suite designed to assess the app-specific penalty computation
// in a scenario where a peer with a staked identity and valid subscriptions has a spam record. The suite runs multiple
// sub-tests, each targeting a specific type of control message (graft, prune, ihave, iwant, RpcPublishMessage). The focus
// is on the impact of spam records on the app-specific score, specifically how such records negate the default reward
// a staked peer would otherwise receive, leaving only the penalty as the app-specific score. This testing reflects the
// asynchronous nature of app-specific score updates in GossipSub's cache.
func TestScoreRegistry_PeerWithSpamRecord(t *testing.T) {
	t.Run("graft", func(t *testing.T) {
		testScoreRegistryPeerWithSpamRecord(t, p2pmsg.CtrlMsgGraft, penaltyValueFixtures().GraftMisbehaviour)
	})
	t.Run("prune", func(t *testing.T) {
		testScoreRegistryPeerWithSpamRecord(t, p2pmsg.CtrlMsgPrune, penaltyValueFixtures().PruneMisbehaviour)
	})
	t.Run("ihave", func(t *testing.T) {
		testScoreRegistryPeerWithSpamRecord(t, p2pmsg.CtrlMsgIHave, penaltyValueFixtures().IHaveMisbehaviour)
	})
	t.Run("iwant", func(t *testing.T) {
		testScoreRegistryPeerWithSpamRecord(t, p2pmsg.CtrlMsgIWant, penaltyValueFixtures().IWantMisbehaviour)
	})
	t.Run("RpcPublishMessage", func(t *testing.T) {
		testScoreRegistryPeerWithSpamRecord(t, p2pmsg.RpcPublishMessage, penaltyValueFixtures().PublishMisbehaviour)
	})
}

// testScoreRegistryPeerWithSpamRecord conducts an individual test within the TestScoreRegistry_PeerWithSpamRecord suite.
// It evaluates the ScoreRegistry's handling of a staked peer with valid subscriptions when a spam record is present for
// the peer ID. The function simulates the process of starting the registry, recording a misbehavior, and then verifying the
// updates to the spam records and app-specific score cache based on the type of control message received.
// Parameters:
// - t *testing.T: The test context.
// - messageType p2pmsg.ControlMessageType: The type of control message being tested.
// - expectedPenalty float64: The expected penalty value for the given control message type.
// This function specifically tests how the ScoreRegistry updates a peer's app-specific score in response to spam records,
// emphasizing the removal of the default reward for staked peers with valid roles and focusing on the asynchronous update
// mechanism of the app-specific score in the cache.
func testScoreRegistryPeerWithSpamRecord(t *testing.T, messageType p2pmsg.ControlMessageType, expectedPenalty float64) {
	peerID := peer.ID("peer-1")

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)
	// refresh cached app-specific score every 100 milliseconds to speed up the test.
	cfg.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters.AppSpecificScore.ScoreTTL = 10 * time.Millisecond

	reg, spamRecords, appScoreCache := newGossipSubAppSpecificScoreRegistry(t,
		cfg.NetworkConfig.GossipSub.ScoringParameters,
		scoring.InitAppScoreRecordStateFunc(cfg.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters.SpamRecordCache.Decay.MaximumSpamPenaltyDecayFactor),
		withStakedIdentities(peerID),
		withValidSubscriptions(peerID))

	// starts the registry.
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	reg.Start(signalerCtx)
	unittest.RequireCloseBefore(t, reg.Ready(), 1*time.Second, "failed to start GossipSubAppSpecificScoreRegistry")

	// initially, the spamRecords should not have the peer id; also the app specific score record should not be in the cache.
	require.False(t, spamRecords.Has(peerID))
	score, updated, exists := appScoreCache.Get(peerID) // get the score from the cache.
	require.False(t, exists)
	require.Equal(t, time.Time{}, updated)
	require.Equal(t, float64(0), score)

	scoreOptParameters := cfg.NetworkConfig.GossipSub.ScoringParameters.PeerScoring.Protocol.AppSpecificScore

	// eventually, the app specific score should be updated in the cache.
	require.Eventually(t, func() bool {
		// calling the app specific score function when there is no app specific score in the cache should eventually update the cache.
		score := reg.AppSpecificScoreFunc()(peerID)
		// since the peer id does not have a spam record, the app specific score should be the max app specific reward, which
		// is the default reward for a staked peer that has valid subscriptions.
		return scoreOptParameters.MaxAppSpecificReward == score
	}, 5*time.Second, 100*time.Millisecond)

	// report a misbehavior for the peer id.
	reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
		PeerID:  peerID,
		MsgType: messageType,
		Errors:  p2p.InvCtrlMsgErrs{p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid graft"), p2p.CtrlMsgNonClusterTopicType)},
	})

	// the penalty should now be updated in the spamRecords
	record, err, ok := spamRecords.Get(peerID) // get the record from the spamRecords.
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Less(t, math.Abs(expectedPenalty-record.Penalty), 10e-3)                                                                                                                                         // penalty should be updated to -10.
	assert.Equal(t, scoring.InitAppScoreRecordStateFunc(cfg.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters.SpamRecordCache.Decay.MaximumSpamPenaltyDecayFactor)().Decay, record.Decay) // decay should be initialized to the initial state.

	queryTime := time.Now()
	// eventually, the app specific score should be updated in the cache.
	require.Eventually(t, func() bool {
		// calling the app specific score function when there is no app specific score in the cache should eventually update the cache.
		score := reg.AppSpecificScoreFunc()(peerID)
		// this peer has a spam record, with no subscription penalty. Hence, the app specific score should only be the spam penalty,
		// and the peer should be deprived of the default reward for its valid staked role.
		// As the app specific score in the cache and spam penalty in the spamRecords are updated at different times, we account for 0.1% error.
		return math.Abs(expectedPenalty-score)/math.Max(expectedPenalty, score) < 0.001
	}, 5*time.Second, 100*time.Millisecond)

	// the app specific score should now be updated in the cache.
	score, updated, exists = appScoreCache.Get(peerID) // get the score from the cache.
	require.True(t, exists)
	require.True(t, updated.After(queryTime))
	require.True(t, math.Abs(expectedPenalty-score)/math.Max(expectedPenalty, score) < 0.001)

	// stop the registry.
	cancel()
	unittest.RequireCloseBefore(t, reg.Done(), 1*time.Second, "failed to stop GossipSubAppSpecificScoreRegistry")
}

// TestScoreRegistry_SpamRecordWithUnknownIdentity is a test suite for verifying the behavior of the ScoreRegistry
// when handling spam records associated with unknown identities. It tests various scenarios based on different control
// message types, including graft, prune, ihave, iwant, and RpcPublishMessage. Each sub-test validates the app-specific
// penalty computation and updates to the score registry when a peer with an unknown identity sends these control messages.
func TestScoreRegistry_SpamRecordWithUnknownIdentity(t *testing.T) {
	t.Run("graft", func(t *testing.T) {
		testScoreRegistrySpamRecordWithUnknownIdentity(t, p2pmsg.CtrlMsgGraft, penaltyValueFixtures().GraftMisbehaviour)
	})
	t.Run("prune", func(t *testing.T) {
		testScoreRegistrySpamRecordWithUnknownIdentity(t, p2pmsg.CtrlMsgPrune, penaltyValueFixtures().PruneMisbehaviour)
	})
	t.Run("ihave", func(t *testing.T) {
		testScoreRegistrySpamRecordWithUnknownIdentity(t, p2pmsg.CtrlMsgIHave, penaltyValueFixtures().IHaveMisbehaviour)
	})
	t.Run("iwant", func(t *testing.T) {
		testScoreRegistrySpamRecordWithUnknownIdentity(t, p2pmsg.CtrlMsgIWant, penaltyValueFixtures().IWantMisbehaviour)
	})
	t.Run("RpcPublishMessage", func(t *testing.T) {
		testScoreRegistrySpamRecordWithUnknownIdentity(t, p2pmsg.RpcPublishMessage, penaltyValueFixtures().PublishMisbehaviour)
	})
}

// testScoreRegistrySpamRecordWithUnknownIdentity tests the app-specific penalty computation of the node when there
// is a spam record for a peer ID with an unknown identity. It examines the functionality of the GossipSubAppSpecificScoreRegistry
// under various conditions, including the initialization state, spam record creation, and the impact of different control message types.
// Parameters:
// - t *testing.T: The testing context.
// - messageType p2pmsg.ControlMessageType: The type of control message being tested.
// - expectedPenalty float64: The expected penalty value for the given control message type.
// The function simulates the process of starting the registry, reporting a misbehavior for the peer ID, and verifying the
// updates to the spam records and app-specific score cache. It ensures that the penalties are correctly computed and applied
// based on the given control message type and the state of the peer ID (unknown identity and spam record presence).
func testScoreRegistrySpamRecordWithUnknownIdentity(t *testing.T, messageType p2pmsg.ControlMessageType, expectedPenalty float64) {
	peerID := peer.ID("peer-1")
	cfg, err := config.DefaultConfig()
	require.NoError(t, err)
	// refresh cached app-specific score every 100 milliseconds to speed up the test.
	cfg.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters.AppSpecificScore.ScoreTTL = 100 * time.Millisecond

	reg, spamRecords, appScoreCache := newGossipSubAppSpecificScoreRegistry(t,
		cfg.NetworkConfig.GossipSub.ScoringParameters,
		scoring.InitAppScoreRecordStateFunc(cfg.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters.SpamRecordCache.Decay.MaximumSpamPenaltyDecayFactor),
		withUnknownIdentity(peerID),
		withValidSubscriptions(peerID))

	// starts the registry.
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	reg.Start(signalerCtx)
	unittest.RequireCloseBefore(t, reg.Ready(), 1*time.Second, "failed to start GossipSubAppSpecificScoreRegistry")

	// initially, the spamRecords should not have the peer id; also the app specific score record should not be in the cache.
	require.False(t, spamRecords.Has(peerID))
	score, updated, exists := appScoreCache.Get(peerID) // get the score from the cache.
	require.False(t, exists)
	require.Equal(t, time.Time{}, updated)
	require.Equal(t, float64(0), score)

	scoreOptParameters := cfg.NetworkConfig.GossipSub.ScoringParameters.PeerScoring.Protocol.AppSpecificScore

	// eventually the app specific score should be updated in the cache to the penalty value for unknown identity.
	require.Eventually(t, func() bool {
		// calling the app specific score function when there is no app specific score in the cache should eventually update the cache.
		score := reg.AppSpecificScoreFunc()(peerID)
		// peer does not have spam record, but has an unknown identity. Hence, the app specific score should be the staking penalty.
		return scoreOptParameters.UnknownIdentityPenalty == score
	}, 5*time.Second, 100*time.Millisecond)

	// report a misbehavior for the peer id.
	reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
		PeerID:  peerID,
		MsgType: messageType,
		Errors:  p2p.InvCtrlMsgErrs{p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid graft"), p2p.CtrlMsgNonClusterTopicType)},
	})
	// the penalty should now be updated.
	record, err, ok := spamRecords.Get(peerID) // get the record from the spamRecords.
	require.True(t, ok)
	require.NoError(t, err)
	require.Less(t, math.Abs(expectedPenalty-record.Penalty), 10e-3)                                                                                                                                         // penalty should be updated to -10, we account for decay.
	require.Equal(t, scoring.InitAppScoreRecordStateFunc(cfg.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters.SpamRecordCache.Decay.MaximumSpamPenaltyDecayFactor)().Decay, record.Decay) // decay should be initialized to the initial state.

	queryTime := time.Now()
	// eventually, the app specific score should be updated in the cache.
	require.Eventually(t, func() bool {
		// calling the app specific score function when there is no app specific score in the cache should eventually update the cache.
		score := reg.AppSpecificScoreFunc()(peerID)
		// the peer has spam record as well as an unknown identity. Hence, the app specific score should be the spam penalty
		// and the staking penalty.
		// As the app specific score in the cache and spam penalty in the spamRecords are updated at different times, we account for 0.1% error.
		return unittest.AreNumericallyClose(expectedPenalty+scoreOptParameters.UnknownIdentityPenalty, score, 0.01)
	}, 5*time.Second, 10*time.Millisecond)

	// the app specific score should now be updated in the cache.
	score, updated, exists = appScoreCache.Get(peerID) // get the score from the cache.
	require.True(t, exists)
	require.True(t, updated.After(queryTime))

	unittest.RequireNumericallyClose(t, expectedPenalty+scoreOptParameters.UnknownIdentityPenalty, score, 0.01)
	assert.Equal(t, scoring.InitAppScoreRecordStateFunc(cfg.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters.SpamRecordCache.Decay.MaximumSpamPenaltyDecayFactor)().Decay, record.Decay) // decay should be initialized to the initial state.

	// stop the registry.
	cancel()
	unittest.RequireCloseBefore(t, reg.Done(), 1*time.Second, "failed to stop GossipSubAppSpecificScoreRegistry")
}

// TestScoreRegistry_SpamRecordWithSubscriptionPenalty is a test suite for verifying the behavior of the ScoreRegistry
// in handling spam records associated with invalid subscriptions. It encompasses a series of sub-tests, each focusing on
// a different control message type: graft, prune, ihave, iwant, and RpcPublishMessage. These sub-tests are designed to
// validate the appropriate application of penalties in the ScoreRegistry when a peer with an invalid subscription is involved
// in spam activities, as indicated by these control messages.
func TestScoreRegistry_SpamRecordWithSubscriptionPenalty(t *testing.T) {
	t.Run("graft", func(t *testing.T) {
		testScoreRegistrySpamRecordWithSubscriptionPenalty(t, p2pmsg.CtrlMsgGraft, penaltyValueFixtures().GraftMisbehaviour)
	})
	t.Run("prune", func(t *testing.T) {
		testScoreRegistrySpamRecordWithSubscriptionPenalty(t, p2pmsg.CtrlMsgPrune, penaltyValueFixtures().PruneMisbehaviour)
	})
	t.Run("ihave", func(t *testing.T) {
		testScoreRegistrySpamRecordWithSubscriptionPenalty(t, p2pmsg.CtrlMsgIHave, penaltyValueFixtures().IHaveMisbehaviour)
	})
	t.Run("iwant", func(t *testing.T) {
		testScoreRegistrySpamRecordWithSubscriptionPenalty(t, p2pmsg.CtrlMsgIWant, penaltyValueFixtures().IWantMisbehaviour)
	})
	t.Run("RpcPublishMessage", func(t *testing.T) {
		testScoreRegistrySpamRecordWithSubscriptionPenalty(t, p2pmsg.RpcPublishMessage, penaltyValueFixtures().PublishMisbehaviour)
	})
}

// testScoreRegistrySpamRecordWithSubscriptionPenalty tests the application-specific penalty computation in the ScoreRegistry
// when a spam record exists for a peer ID that also has an invalid subscription. The function simulates the process of
// initializing the registry, handling spam records, and updating penalties based on various control message types.
// Parameters:
// - t *testing.T: The testing context.
// - messageType p2pmsg.ControlMessageType: The type of control message being tested.
// - expectedPenalty float64: The expected penalty value for the given control message type.
// The function focuses on evaluating the registry's response to spam activities (as represented by control messages) from a
// peer with invalid subscriptions. It verifies that penalties are accurately computed and applied, taking into account both
// the spam record and the invalid subscription status of the peer.
func testScoreRegistrySpamRecordWithSubscriptionPenalty(t *testing.T, messageType p2pmsg.ControlMessageType, expectedPenalty float64) {
	peerID := peer.ID("peer-1")
	cfg, err := config.DefaultConfig()
	require.NoError(t, err)
	// refresh cached app-specific score every 100 milliseconds to speed up the test.
	cfg.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters.AppSpecificScore.ScoreTTL = 100 * time.Millisecond

	reg, spamRecords, appScoreCache := newGossipSubAppSpecificScoreRegistry(t,
		cfg.NetworkConfig.GossipSub.ScoringParameters,
		scoring.InitAppScoreRecordStateFunc(cfg.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters.SpamRecordCache.Decay.MaximumSpamPenaltyDecayFactor),
		withStakedIdentities(peerID),
		withInvalidSubscriptions(peerID))

	// starts the registry.
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	reg.Start(signalerCtx)
	unittest.RequireCloseBefore(t, reg.Ready(), 1*time.Second, "failed to start GossipSubAppSpecificScoreRegistry")

	// initially, the spamRecords should not have the peer id; also the app specific score record should not be in the cache.
	require.False(t, spamRecords.Has(peerID))
	score, updated, exists := appScoreCache.Get(peerID) // get the score from the cache.
	require.False(t, exists)
	require.Equal(t, time.Time{}, updated)
	require.Equal(t, float64(0), score)

	scoreOptParameters := cfg.NetworkConfig.GossipSub.ScoringParameters.PeerScoring.Protocol.AppSpecificScore

	// peer does not have spam record, but has invalid subscription. Hence, the app specific score should be subscription penalty.
	// eventually the app specific score should be updated in the cache to the penalty value for subscription penalty.
	require.Eventually(t, func() bool {
		// calling the app specific score function when there is no app specific score in the cache should eventually update the cache.
		score := reg.AppSpecificScoreFunc()(peerID)
		// peer does not have spam record, but has an invalid subscription penalty.
		return scoreOptParameters.InvalidSubscriptionPenalty == score
	}, 5*time.Second, 100*time.Millisecond)

	// report a misbehavior for the peer id.
	reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
		PeerID:  peerID,
		MsgType: messageType,
		Errors:  p2p.InvCtrlMsgErrs{p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid graft"), p2p.CtrlMsgNonClusterTopicType)},
	})
	// the penalty should now be updated.
	record, err, ok := spamRecords.Get(peerID) // get the record from the spamRecords.
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Less(t, math.Abs(expectedPenalty-record.Penalty), 10e-3)
	assert.Equal(t, scoring.InitAppScoreRecordStateFunc(cfg.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters.SpamRecordCache.Decay.MaximumSpamPenaltyDecayFactor)().Decay, record.Decay) // decay should be initialized to the initial state.

	queryTime := time.Now()
	// eventually, the app specific score should be updated in the cache.
	require.Eventually(t, func() bool {
		// calling the app specific score function when there is no app specific score in the cache should eventually update the cache.
		score := reg.AppSpecificScoreFunc()(peerID)
		// the peer has spam record as well as an unknown identity. Hence, the app specific score should be the spam penalty
		// and the staking penalty.
		// As the app specific score in the cache and spam penalty in the spamRecords are updated at different times, we account for 0.1% error.
		return unittest.AreNumericallyClose(expectedPenalty+scoreOptParameters.InvalidSubscriptionPenalty, score, 0.01)
	}, 5*time.Second, 10*time.Millisecond)

	// the app specific score should now be updated in the cache.
	score, updated, exists = appScoreCache.Get(peerID) // get the score from the cache.
	require.True(t, exists)
	require.True(t, updated.After(queryTime))
	unittest.RequireNumericallyClose(t, expectedPenalty+scoreOptParameters.InvalidSubscriptionPenalty, score, 0.01)

	// stop the registry.
	cancel()
	unittest.RequireCloseBefore(t, reg.Done(), 1*time.Second, "failed to stop GossipSubAppSpecificScoreRegistry")
}

// TestSpamPenaltyDecaysInCache tests that the spam penalty records decay over time in the cache.
func TestScoreRegistry_SpamPenaltyDecaysInCache(t *testing.T) {
	peerID := peer.ID("peer-1")
	cfg, err := config.DefaultConfig()
	require.NoError(t, err)
	// refresh cached app-specific score every 100 milliseconds to speed up the test.
	cfg.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters.AppSpecificScore.ScoreTTL = 100 * time.Millisecond

	reg, _, _ := newGossipSubAppSpecificScoreRegistry(t,
		cfg.NetworkConfig.GossipSub.ScoringParameters,
		scoring.InitAppScoreRecordStateFunc(cfg.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters.SpamRecordCache.Decay.MaximumSpamPenaltyDecayFactor),
		withStakedIdentities(peerID),
		withValidSubscriptions(peerID))
	// starts the registry.
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	reg.Start(signalerCtx)
	unittest.RequireCloseBefore(t, reg.Ready(), 1*time.Second, "failed to start GossipSubAppSpecificScoreRegistry")

	// report a misbehavior for the peer id.
	reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
		PeerID:  peerID,
		MsgType: p2pmsg.CtrlMsgPrune,
		Errors:  p2p.InvCtrlMsgErrs{p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid graft"), p2p.CtrlMsgNonClusterTopicType)},
	})

	time.Sleep(1 * time.Second) // wait for the penalty to decay.

	reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
		PeerID:  peerID,
		MsgType: p2pmsg.CtrlMsgGraft,
		Errors:  p2p.InvCtrlMsgErrs{p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid graft"), p2p.CtrlMsgNonClusterTopicType)},
	})

	time.Sleep(1 * time.Second) // wait for the penalty to decay.

	reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
		PeerID:  peerID,
		MsgType: p2pmsg.CtrlMsgIHave,
		Errors:  p2p.InvCtrlMsgErrs{p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid graft"), p2p.CtrlMsgNonClusterTopicType)},
	})

	time.Sleep(1 * time.Second) // wait for the penalty to decay.

	reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
		PeerID:  peerID,
		MsgType: p2pmsg.CtrlMsgIWant,
		Errors:  p2p.InvCtrlMsgErrs{p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid graft"), p2p.CtrlMsgNonClusterTopicType)},
	})

	time.Sleep(1 * time.Second) // wait for the penalty to decay.

	reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
		PeerID:  peerID,
		MsgType: p2pmsg.RpcPublishMessage,
		Errors:  p2p.InvCtrlMsgErrs{p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid graft"), p2p.CtrlMsgNonClusterTopicType)},
	})

	time.Sleep(1 * time.Second) // wait for the penalty to decay.

	// the upper bound is the sum of the penalties without decay.
	scoreUpperBound := penaltyValueFixtures().PruneMisbehaviour +
		penaltyValueFixtures().GraftMisbehaviour +
		penaltyValueFixtures().IHaveMisbehaviour +
		penaltyValueFixtures().IWantMisbehaviour +
		penaltyValueFixtures().PublishMisbehaviour
	// the lower bound is the sum of the penalties with decay assuming the decay is applied 4 times to the sum of the penalties.
	// in reality, the decay is applied 4 times to the first penalty, then 3 times to the second penalty, and so on.
	r := scoring.InitAppScoreRecordStateFunc(cfg.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters.SpamRecordCache.Decay.MaximumSpamPenaltyDecayFactor)()
	scoreLowerBound := scoreUpperBound * math.Pow(r.Decay, 4)

	// eventually, the app specific score should be updated in the cache.
	require.Eventually(t, func() bool {
		// when the app specific penalty function is called for the first time, the decay functionality should be kicked in
		// the cache, and the penalty should be updated. Note that since the penalty values are negative, the default staked identity
		// reward is not applied. Hence, the penalty is only comprised of the penalties.
		score := reg.AppSpecificScoreFunc()(peerID)
		// with decay, the penalty should be between the upper and lower bounds.
		return score > scoreUpperBound && score < scoreLowerBound
	}, 5*time.Second, 100*time.Millisecond)

	// stop the registry.
	cancel()
	unittest.RequireCloseBefore(t, reg.Done(), 1*time.Second, "failed to stop GossipSubAppSpecificScoreRegistry")
}

// TestSpamPenaltyDecayToZero tests that the spam penalty decays to zero over time, and when the spam penalty of
// a peer is set back to zero, its app specific penalty is also reset to the initial state.
func TestScoreRegistry_SpamPenaltyDecayToZero(t *testing.T) {
	peerID := peer.ID("peer-1")
	cfg, err := config.DefaultConfig()
	require.NoError(t, err)
	// refresh cached app-specific score every 100 milliseconds to speed up the test.
	cfg.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters.AppSpecificScore.ScoreTTL = 100 * time.Millisecond

	reg, spamRecords, _ := newGossipSubAppSpecificScoreRegistry(t,
		cfg.NetworkConfig.GossipSub.ScoringParameters,
		func() p2p.GossipSubSpamRecord {
			return p2p.GossipSubSpamRecord{
				Decay:   0.02, // we choose a small decay value to speed up the test.
				Penalty: 0,
			}
		},
		withStakedIdentities(peerID),
		withValidSubscriptions(peerID))

	// starts the registry.
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	reg.Start(signalerCtx)
	unittest.RequireCloseBefore(t, reg.Ready(), 1*time.Second, "failed to start GossipSubAppSpecificScoreRegistry")

	scoreOptParameters := cfg.NetworkConfig.GossipSub.ScoringParameters.PeerScoring.Protocol.AppSpecificScore

	// report a misbehavior for the peer id.
	reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
		PeerID:  peerID,
		MsgType: p2pmsg.CtrlMsgGraft,
		Errors:  p2p.InvCtrlMsgErrs{p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid graft"), p2p.CtrlMsgNonClusterTopicType)},
	})

	// decays happen every second, so we wait for 1 second to make sure the penalty is updated.
	time.Sleep(1 * time.Second)
	// the penalty should now be updated, it should be still negative but greater than the penalty value (due to decay).
	require.Eventually(t, func() bool {
		score := reg.AppSpecificScoreFunc()(peerID)
		// the penalty should be less than zero and greater than the penalty value (due to decay).
		return score < 0 && score > penaltyValueFixtures().GraftMisbehaviour
	}, 5*time.Second, 100*time.Millisecond)

	require.Eventually(t, func() bool {
		// the spam penalty should eventually decay to zero.
		r, err, ok := spamRecords.Get(peerID)
		return ok && err == nil && r.Penalty == 0.0
	}, 5*time.Second, 100*time.Millisecond)

	require.Eventually(t, func() bool {
		// when the spam penalty is decayed to zero, the app specific penalty of the node should reset back to default staking reward.
		return reg.AppSpecificScoreFunc()(peerID) == scoreOptParameters.StakedIdentityReward
	}, 5*time.Second, 100*time.Millisecond)

	// the penalty should now be zero.
	record, err, ok := spamRecords.Get(peerID) // get the record from the spamRecords.
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Equal(t, 0.0, record.Penalty) // penalty should be zero.

	// stop the registry.
	cancel()
	unittest.RequireCloseBefore(t, reg.Done(), 1*time.Second, "failed to stop GossipSubAppSpecificScoreRegistry")
}

// TestPersistingUnknownIdentityPenalty tests that even though the spam penalty is decayed to zero, the unknown identity penalty
// is persisted. This is because the unknown identity penalty is not decayed.
func TestScoreRegistry_PersistingUnknownIdentityPenalty(t *testing.T) {
	peerID := peer.ID("peer-1")

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)
	// refresh cached app-specific score every 100 milliseconds to speed up the test.
	cfg.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters.AppSpecificScore.ScoreTTL = 100 * time.Millisecond

	reg, spamRecords, _ := newGossipSubAppSpecificScoreRegistry(t,
		cfg.NetworkConfig.GossipSub.ScoringParameters,
		func() p2p.GossipSubSpamRecord {
			return p2p.GossipSubSpamRecord{
				Decay:   0.02, // we choose a small decay value to speed up the test.
				Penalty: 0,
			}
		},
		withUnknownIdentity(peerID), // the peer id has an unknown identity.
		withValidSubscriptions(peerID))

	// starts the registry.
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	reg.Start(signalerCtx)
	unittest.RequireCloseBefore(t, reg.Ready(), 1*time.Second, "failed to start GossipSubAppSpecificScoreRegistry")

	scoreOptParameters := cfg.NetworkConfig.GossipSub.ScoringParameters.PeerScoring.Protocol.AppSpecificScore

	// initially, the app specific score should be the default unknown identity penalty.
	require.Eventually(t, func() bool {
		score := reg.AppSpecificScoreFunc()(peerID)
		return score == scoreOptParameters.UnknownIdentityPenalty
	}, 5*time.Second, 100*time.Millisecond)

	// report a misbehavior for the peer id.
	reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
		PeerID:  peerID,
		MsgType: p2pmsg.CtrlMsgGraft,
		Errors:  p2p.InvCtrlMsgErrs{p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid graft"), p2p.CtrlMsgNonClusterTopicType)},
	})

	// decays happen every second, so we wait for 1 second to make sure the penalty is updated.
	time.Sleep(1 * time.Second)

	// the penalty should now be updated, it should be still negative but greater than the penalty value (due to decay).
	require.Eventually(t, func() bool {
		score := reg.AppSpecificScoreFunc()(peerID)
		// Ideally, the score should be the sum of the default invalid subscription penalty and the graft penalty, however,
		// due to exponential decay of the spam penalty and asynchronous update the app specific score; score should be in the range of [scoring.
		// (scoring.DefaultUnknownIdentityPenalty+penaltyValueFixtures().GraftMisbehaviour, scoring.DefaultUnknownIdentityPenalty).
		return score < scoreOptParameters.UnknownIdentityPenalty && score > scoreOptParameters.UnknownIdentityPenalty+penaltyValueFixtures().GraftMisbehaviour
	}, 5*time.Second, 100*time.Millisecond)

	require.Eventually(t, func() bool {
		// the spam penalty should eventually decay to zero.
		r, err, ok := spamRecords.Get(peerID)
		return ok && err == nil && r.Penalty == 0.0
	}, 5*time.Second, 100*time.Millisecond)

	require.Eventually(t, func() bool {
		// when the spam penalty is decayed to zero, the app specific penalty of the node should only contain the unknown identity penalty.
		return reg.AppSpecificScoreFunc()(peerID) == scoreOptParameters.UnknownIdentityPenalty
	}, 5*time.Second, 100*time.Millisecond)

	// the spam penalty should now be zero in spamRecords.
	record, err, ok := spamRecords.Get(peerID) // get the record from the spamRecords.
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Equal(t, 0.0, record.Penalty) // penalty should be zero.

	// stop the registry.
	cancel()
	unittest.RequireCloseBefore(t, reg.Done(), 1*time.Second, "failed to stop GossipSubAppSpecificScoreRegistry")
}

// TestPersistingInvalidSubscriptionPenalty tests that even though the spam penalty is decayed to zero, the invalid subscription penalty
// is persisted. This is because the invalid subscription penalty is not decayed.
func TestScoreRegistry_PersistingInvalidSubscriptionPenalty(t *testing.T) {
	peerID := peer.ID("peer-1")

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)
	// refresh cached app-specific score every 100 milliseconds to speed up the test.
	cfg.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters.AppSpecificScore.ScoreTTL = 100 * time.Millisecond

	reg, spamRecords, _ := newGossipSubAppSpecificScoreRegistry(t,
		cfg.NetworkConfig.GossipSub.ScoringParameters,
		func() p2p.GossipSubSpamRecord {
			return p2p.GossipSubSpamRecord{
				Decay:   0.02, // we choose a small decay value to speed up the test.
				Penalty: 0,
			}
		},
		withStakedIdentities(peerID),
		withInvalidSubscriptions(peerID)) // the peer id has an invalid subscription

	// starts the registry.
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	reg.Start(signalerCtx)
	unittest.RequireCloseBefore(t, reg.Ready(), 1*time.Second, "failed to start GossipSubAppSpecificScoreRegistry")

	scoreOptParameters := cfg.NetworkConfig.GossipSub.ScoringParameters.PeerScoring.Protocol.AppSpecificScore

	// initially, the app specific score should be the default invalid subscription penalty.
	require.Eventually(t, func() bool {
		score := reg.AppSpecificScoreFunc()(peerID)
		return score == scoreOptParameters.InvalidSubscriptionPenalty
	}, 5*time.Second, 100*time.Millisecond)

	// report a misbehavior for the peer id.
	reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
		PeerID:  peerID,
		MsgType: p2pmsg.CtrlMsgGraft,
		Errors:  p2p.InvCtrlMsgErrs{p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid graft"), p2p.CtrlMsgNonClusterTopicType)},
	})
	// with reported spam, the app specific score should be the default invalid subscription penalty + the spam penalty.
	require.Eventually(t, func() bool {
		score := reg.AppSpecificScoreFunc()(peerID)
		// Ideally, the score should be the sum of the default invalid subscription penalty and the graft penalty, however,
		// due to exponential decay of the spam penalty and asynchronous update the app specific score; score should be in the range of [scoring.
		// (DefaultInvalidSubscriptionPenalty+penaltyValueFixtures().GraftMisbehaviour, scoring.DefaultInvalidSubscriptionPenalty).
		return score < scoreOptParameters.InvalidSubscriptionPenalty && score > scoreOptParameters.InvalidSubscriptionPenalty+penaltyValueFixtures().GraftMisbehaviour
	}, 5*time.Second, 100*time.Millisecond)

	require.Eventually(t, func() bool {
		// the spam penalty should eventually decay to zero.
		r, err, ok := spamRecords.Get(peerID)
		return ok && err == nil && r.Penalty == 0.0
	}, 5*time.Second, 100*time.Millisecond)

	require.Eventually(t, func() bool {
		// when the spam penalty is decayed to zero, the app specific penalty of the node should only contain the default invalid subscription penalty.
		return reg.AppSpecificScoreFunc()(peerID) == scoreOptParameters.UnknownIdentityPenalty
	}, 5*time.Second, 100*time.Millisecond)

	// the spam penalty should now be zero in spamRecords.
	record, err, ok := spamRecords.Get(peerID) // get the record from the spamRecords.
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Equal(t, 0.0, record.Penalty) // penalty should be zero.

	// stop the registry.
	cancel()
	unittest.RequireCloseBefore(t, reg.Done(), 1*time.Second, "failed to stop GossipSubAppSpecificScoreRegistry")
}

// TestScoreRegistry_TestSpamRecordDecayAdjustment ensures that spam record decay is increased each time a peers score reaches the scoring.IncreaseDecayThreshold eventually
// sustained misbehavior will result in the spam record decay reaching the minimum decay speed .99, and the decay speed is reset to the max decay speed .8.
func TestScoreRegistry_TestSpamRecordDecayAdjustment(t *testing.T) {
	cfg, err := config.DefaultConfig()
	require.NoError(t, err)
	// refresh cached app-specific score every 100 milliseconds to speed up the test.
	cfg.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters.AppSpecificScore.ScoreTTL = 100 * time.Millisecond
	// increase configured DecayRateReductionFactor so that the decay time is increased faster
	cfg.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters.SpamRecordCache.Decay.DecayRateReductionFactor = .1
	cfg.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters.SpamRecordCache.Decay.PenaltyDecayEvaluationPeriod = time.Second

	peer1 := unittest.PeerIdFixture(t)
	peer2 := unittest.PeerIdFixture(t)
	reg, spamRecords, _ := newGossipSubAppSpecificScoreRegistry(t,
		cfg.NetworkConfig.GossipSub.ScoringParameters,
		scoring.InitAppScoreRecordStateFunc(cfg.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters.SpamRecordCache.Decay.MaximumSpamPenaltyDecayFactor),
		withStakedIdentities(peer1, peer2),
		withValidSubscriptions(peer1, peer2))

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	reg.Start(signalerCtx)
	unittest.RequireCloseBefore(t, reg.Ready(), 1*time.Second, "failed to start GossipSubAppSpecificScoreRegistry")

	// initially, the spamRecords should not have the peer ids.
	assert.False(t, spamRecords.Has(peer1))
	assert.False(t, spamRecords.Has(peer2))

	scoreOptParameters := cfg.NetworkConfig.GossipSub.ScoringParameters.PeerScoring.Protocol.AppSpecificScore
	scoringRegistryParameters := cfg.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters
	// since the both peers do not have a spam record, their app specific score should be the max app specific reward, which
	// is the default reward for a staked peer that has valid subscriptions.
	require.Eventually(t, func() bool {
		// when the spam penalty is decayed to zero, the app specific penalty of the node should only contain the unknown identity penalty.
		return scoreOptParameters.MaxAppSpecificReward == reg.AppSpecificScoreFunc()(peer1) && scoreOptParameters.MaxAppSpecificReward == reg.AppSpecificScoreFunc()(peer2)
	}, 5*time.Second, 100*time.Millisecond)

	// simulate sustained malicious activity from peer1, eventually the decay speed
	// for a spam record should be reduced to the MinimumSpamPenaltyDecayFactor
	prevDecay := scoringRegistryParameters.SpamRecordCache.Decay.MaximumSpamPenaltyDecayFactor
	tolerance := 0.1
	require.Eventually(t, func() bool {
		errCount := 500
		errs := make(p2p.InvCtrlMsgErrs, errCount)
		for i := 0; i < errCount; i++ {
			errs[i] = p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid prune"), p2p.CtrlMsgNonClusterTopicType)
		}
		reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
			PeerID:  peer1,
			MsgType: p2pmsg.CtrlMsgPrune,
			Errors:  errs,
		})
		record, err, ok := spamRecords.Get(peer1)
		require.NoError(t, err)
		require.True(t, ok)
		assert.Less(t, math.Abs(prevDecay-record.Decay), tolerance)
		prevDecay = record.Decay
		return record.Decay == scoringRegistryParameters.SpamRecordCache.Decay.MinimumSpamPenaltyDecayFactor
	}, 5*time.Second, 500*time.Millisecond)

	// initialize a spam record for peer2
	reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
		PeerID:  peer2,
		MsgType: p2pmsg.CtrlMsgPrune,
		Errors:  p2p.InvCtrlMsgErrs{p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid prune"), p2p.CtrlMsgNonClusterTopicType)},
	})
	// reduce penalty and increase Decay to scoring.MinimumSpamPenaltyDecayFactor
	record, err := spamRecords.Adjust(peer2, func(record p2p.GossipSubSpamRecord) p2p.GossipSubSpamRecord {
		record.Penalty = -.1
		record.Decay = scoringRegistryParameters.SpamRecordCache.Decay.MinimumSpamPenaltyDecayFactor
		return record
	})
	require.NoError(t, err)
	require.True(t, record.Decay == scoringRegistryParameters.SpamRecordCache.Decay.MinimumSpamPenaltyDecayFactor)
	require.True(t, record.Penalty == -.1)
	// simulate sustained good behavior from peer 2, each time the spam record is read from the cache
	// using Get method the record penalty will be decayed until it is eventually reset to
	// 0 at this point the decay speed for the record should be reset to MaximumSpamPenaltyDecayFactor
	// eventually after penalty reaches the skipDecaThreshold the record decay will be reset to scoringRegistryParameters.MaximumSpamPenaltyDecayFactor
	require.Eventually(t, func() bool {
		record, err, ok := spamRecords.Get(peer2)
		require.NoError(t, err)
		require.True(t, ok)
		return record.Decay == scoringRegistryParameters.SpamRecordCache.Decay.MaximumSpamPenaltyDecayFactor &&
			record.Penalty == 0 &&
			record.LastDecayAdjustment.IsZero()
	}, 5*time.Second, time.Second)

	// ensure decay can be reduced again after recovery for peerID 2
	require.Eventually(t, func() bool {
		reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
			PeerID:  peer2,
			MsgType: p2pmsg.CtrlMsgPrune,
			Errors:  p2p.InvCtrlMsgErrs{p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid prune"), p2p.CtrlMsgNonClusterTopicType)},
		})
		record, err, ok := spamRecords.Get(peer1)
		require.NoError(t, err)
		require.True(t, ok)
		return record.Decay == scoringRegistryParameters.SpamRecordCache.Decay.MinimumSpamPenaltyDecayFactor
	}, 5*time.Second, 500*time.Millisecond)

	// stop the registry.
	cancel()
	unittest.RequireCloseBefore(t, reg.Done(), 1*time.Second, "failed to stop GossipSubAppSpecificScoreRegistry")
}

// TestPeerSpamPenaltyClusterPrefixed evaluates the application-specific penalty calculation for a node when a spam record is present
// for cluster-prefixed topics. In the case of an invalid control message notification marked as cluster-prefixed,
// the application-specific penalty should be reduced by the default reduction factor. This test verifies the accurate computation
// of the application-specific score under these conditions.
func TestPeerSpamPenaltyClusterPrefixed(t *testing.T) {
	ctlMsgTypes := p2pmsg.ControlMessageTypes()
	peerIds := unittest.PeerIdFixtures(t, len(ctlMsgTypes))

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)
	// refresh cached app-specific score every 100 milliseconds to speed up the test.
	cfg.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters.AppSpecificScore.ScoreTTL = 100 * time.Millisecond

	reg, spamRecords, _ := newGossipSubAppSpecificScoreRegistry(t,
		cfg.NetworkConfig.GossipSub.ScoringParameters,
		scoring.InitAppScoreRecordStateFunc(cfg.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters.SpamRecordCache.Decay.MaximumSpamPenaltyDecayFactor),
		withStakedIdentities(peerIds...),
		withValidSubscriptions(peerIds...))

	// starts the registry.
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	reg.Start(signalerCtx)
	unittest.RequireCloseBefore(t, reg.Ready(), 1*time.Second, "failed to start GossipSubAppSpecificScoreRegistry")

	scoreOptParameters := cfg.NetworkConfig.GossipSub.ScoringParameters.PeerScoring.Protocol.AppSpecificScore

	for _, peerID := range peerIds {
		// initially, the spamRecords should not have the peer id.
		assert.False(t, spamRecords.Has(peerID))
		// since the peer id does not have a spam record, the app specific score should (eventually, due to caching) be the max app specific reward, which
		// is the default reward for a staked peer that has valid subscriptions.
		require.Eventually(t, func() bool {
			// calling the app specific score function when there is no app specific score in the cache should eventually update the cache.
			score := reg.AppSpecificScoreFunc()(peerID)
			// since the peer id does not have a spam record, the app specific score should be the max app specific reward, which
			// is the default reward for a staked peer that has valid subscriptions.
			return score == scoreOptParameters.MaxAppSpecificReward
		}, 5*time.Second, 100*time.Millisecond)

	}

	// Report consecutive misbehavior's for the specified peer ID. Two misbehavior's are reported concurrently:
	// 1. With IsClusterPrefixed set to false, ensuring the penalty applied to the application-specific score is not reduced.
	// 2. With IsClusterPrefixed set to true, reducing the penalty added to the overall app-specific score by the default reduction factor.
	for i, ctlMsgType := range ctlMsgTypes {
		peerID := peerIds[i]
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
				PeerID:  peerID,
				MsgType: ctlMsgType,
				Errors:  p2p.InvCtrlMsgErrs{p2p.NewInvCtrlMsgErr(fmt.Errorf(fmt.Sprintf("invalid %s", ctlMsgType)), p2p.CtrlMsgNonClusterTopicType)},
			})
		}()
		go func() {
			defer wg.Done()
			reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
				PeerID:  peerID,
				MsgType: ctlMsgType,
				Errors:  p2p.InvCtrlMsgErrs{p2p.NewInvCtrlMsgErr(fmt.Errorf(fmt.Sprintf("invalid %s", ctlMsgType)), p2p.CtrlMsgTopicTypeClusterPrefixed)},
			})
		}()
		unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "timed out waiting for goroutines to finish")

		// expected penalty should be penaltyValueFixtures().GraftMisbehaviour * (1  + clusterReductionFactor)
		expectedPenalty := penaltyValueFixture(ctlMsgType) * (1 + penaltyValueFixtures().ClusterPrefixedReductionFactor)

		// the penalty should now be updated in the spamRecords
		record, err, ok := spamRecords.Get(peerID) // get the record from the spamRecords.
		assert.True(t, ok)
		assert.NoError(t, err)
		assert.Less(t, math.Abs(expectedPenalty-record.Penalty), 10e-3)
		assert.Equal(t, scoring.InitAppScoreRecordStateFunc(cfg.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters.SpamRecordCache.Decay.MaximumSpamPenaltyDecayFactor)().Decay, record.Decay)
		// this peer has a spam record, with no subscription penalty. Hence, the app specific score should only be the spam penalty,
		// and the peer should be deprived of the default reward for its valid staked role.
		score := reg.AppSpecificScoreFunc()(peerID)
		tolerance := 10e-3 // 0.1%
		if expectedPenalty == 0 {
			assert.Less(t, math.Abs(expectedPenalty), tolerance)
		} else {
			assert.Less(t, math.Abs(expectedPenalty-score)/expectedPenalty, tolerance)
		}
	}

	// stop the registry.
	cancel()
	unittest.RequireCloseBefore(t, reg.Done(), 1*time.Second, "failed to stop GossipSubAppSpecificScoreRegistry")
}

// TestInvalidControlMessageMultiErrorScoreCalculation tests that invalid control message penalties are calculated as expected when notifications
// contain multiple errors with multiple different severity levels.
func TestInvalidControlMessageMultiErrorScoreCalculation(t *testing.T) {
	peerIds := unittest.PeerIdFixtures(t, 5)
	cfg, err := config.DefaultConfig()
	require.NoError(t, err)
	// refresh cached app-specific score every 100 milliseconds to speed up the test.
	cfg.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters.AppSpecificScore.ScoreTTL = 100 * time.Millisecond
	maximumSpamPenaltyDecayFactor := cfg.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters.SpamRecordCache.Decay.MaximumSpamPenaltyDecayFactor
	reg, spamRecords, _ := newGossipSubAppSpecificScoreRegistry(t,
		cfg.NetworkConfig.GossipSub.ScoringParameters,
		scoring.InitAppScoreRecordStateFunc(maximumSpamPenaltyDecayFactor),
		withStakedIdentities(peerIds...),
		withValidSubscriptions(peerIds...))

	// starts the registry.
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	reg.Start(signalerCtx)
	unittest.RequireCloseBefore(t, reg.Ready(), 1*time.Second, "failed to start GossipSubAppSpecificScoreRegistry")

	for _, peerID := range peerIds {
		require.Eventually(t, func() bool {
			// initially, the spamRecords should not have the peer id.
			assert.False(t, spamRecords.Has(peerID))
			// since the peer id does not have a spam record, the app specific score should be the max app specific reward, which
			// is the default reward for a staked peer that has valid subscriptions.
			score := reg.AppSpecificScoreFunc()(peerID)
			return score == cfg.NetworkConfig.GossipSub.ScoringParameters.PeerScoring.Protocol.AppSpecificScore.MaxAppSpecificReward
		}, 5*time.Second, 500*time.Millisecond)
	}

	type testCase struct {
		notification    *p2p.InvCtrlMsgNotif
		expectedPenalty float64
	}
	penaltyValues := penaltyValueFixtures()
	testCases := []*testCase{
		// single error with, with random severity
		{
			notification: &p2p.InvCtrlMsgNotif{
				PeerID:  peerIds[0],
				MsgType: p2pmsg.CtrlMsgGraft,
				Errors: p2p.InvCtrlMsgErrs{
					p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid graft"), p2p.CtrlMsgNonClusterTopicType),
				},
			},
			expectedPenalty: penaltyValues.GraftMisbehaviour,
		},
		// multiple errors with, with same severity
		{
			notification: &p2p.InvCtrlMsgNotif{
				PeerID:  peerIds[1],
				MsgType: p2pmsg.CtrlMsgPrune,
				Errors: p2p.InvCtrlMsgErrs{
					p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid prune"), p2p.CtrlMsgNonClusterTopicType),
					p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid prune"), p2p.CtrlMsgNonClusterTopicType),
					p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid prune"), p2p.CtrlMsgNonClusterTopicType),
				},
			},
			expectedPenalty: penaltyValues.PruneMisbehaviour * 3,
		},
		// multiple errors with, with random severity's
		{
			notification: &p2p.InvCtrlMsgNotif{
				PeerID:  peerIds[2],
				MsgType: p2pmsg.CtrlMsgIHave,
				Errors: p2p.InvCtrlMsgErrs{
					p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid ihave"), p2p.CtrlMsgNonClusterTopicType),
					p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid ihave"), p2p.CtrlMsgNonClusterTopicType),
					p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid ihave"), p2p.CtrlMsgNonClusterTopicType),
					p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid ihave"), p2p.CtrlMsgNonClusterTopicType),
				},
			},
			expectedPenalty: penaltyValues.IHaveMisbehaviour +
				penaltyValues.IHaveMisbehaviour +
				penaltyValues.IHaveMisbehaviour +
				penaltyValues.IHaveMisbehaviour,
		},
		// multiple errors with, with random severity's iwant
		{
			notification: &p2p.InvCtrlMsgNotif{
				PeerID:  peerIds[3],
				MsgType: p2pmsg.CtrlMsgIWant,
				Errors: p2p.InvCtrlMsgErrs{
					p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid iwant"), p2p.CtrlMsgNonClusterTopicType),
					p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid iwant"), p2p.CtrlMsgNonClusterTopicType),
					p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid iwant"), p2p.CtrlMsgNonClusterTopicType),
					p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid iwant"), p2p.CtrlMsgNonClusterTopicType),
				},
			},
			expectedPenalty: penaltyValues.IWantMisbehaviour +
				penaltyValues.IWantMisbehaviour +
				penaltyValues.IWantMisbehaviour +
				penaltyValues.IWantMisbehaviour,
		},
		// multiple errors with mixed cluster prefixed and non cluster prefixed, with random severity's iwant
		{
			notification: &p2p.InvCtrlMsgNotif{
				PeerID:  peerIds[4],
				MsgType: p2pmsg.CtrlMsgIWant,
				Errors: p2p.InvCtrlMsgErrs{
					p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid iwant"), p2p.CtrlMsgNonClusterTopicType),
					p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid iwant"), p2p.CtrlMsgNonClusterTopicType),
					p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid iwant"), p2p.CtrlMsgTopicTypeClusterPrefixed),
					p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid iwant"), p2p.CtrlMsgTopicTypeClusterPrefixed),
				},
			},
			expectedPenalty: penaltyValues.IWantMisbehaviour +
				penaltyValues.IWantMisbehaviour +
				(penaltyValues.IWantMisbehaviour * penaltyValues.ClusterPrefixedReductionFactor) +
				(penaltyValues.IWantMisbehaviour * penaltyValues.ClusterPrefixedReductionFactor),
		},
	}

	for _, tCase := range testCases {
		// report a misbehavior for the peer id.
		reg.OnInvalidControlMessageNotification(tCase.notification)
		// the penalty should now be updated in the spamRecords
		record, err, ok := spamRecords.Get(tCase.notification.PeerID) // get the record from the spamRecords.
		require.True(t, ok)
		require.NoError(t, err)
		require.Less(t, math.Abs(tCase.expectedPenalty-record.Penalty), 10e-3)                                     // penalty should be updated to -10.
		require.Equal(t, scoring.InitAppScoreRecordStateFunc(maximumSpamPenaltyDecayFactor)().Decay, record.Decay) // decay should be initialized to the initial state.

		require.Eventually(t, func() bool {
			// this peer has a spam record, with no subscription penalty. Hence, the app specific score should only be the spam penalty,
			// and the peer should be deprived of the default reward for its valid staked role.
			score := reg.AppSpecificScoreFunc()(tCase.notification.PeerID)
			tolerance := 10e-2
			return math.Abs(tCase.expectedPenalty-score)/tCase.expectedPenalty < tolerance
		}, 5*time.Second, 500*time.Millisecond)
	}

	// stop the registry.
	cancel()
	unittest.RequireCloseBefore(t, reg.Done(), 1*time.Second, "failed to stop GossipSubAppSpecificScoreRegistry")
}

// withStakedIdentities returns a function that sets the identity provider to return staked identities for the given peer ids.
// It is used for testing purposes, and causes the given peer id to benefit from the staked identity reward in GossipSub.
func withStakedIdentities(peerIds ...peer.ID) func(cfg *scoring.GossipSubAppSpecificScoreRegistryConfig) {
	return func(cfg *scoring.GossipSubAppSpecificScoreRegistryConfig) {
		cfg.IdProvider.(*mock.IdentityProvider).On("ByPeerID", testifymock.AnythingOfType("peer.ID")).
			Return(func(pid peer.ID) *flow.Identity {
				for _, peerID := range peerIds {
					if peerID == pid {
						return unittest.IdentityFixture()
					}
				}
				return nil
			}, func(pid peer.ID) bool {
				for _, peerID := range peerIds {
					if peerID == pid {
						return true
					}
				}
				return false
			}).Maybe()
	}
}

// withValidSubscriptions returns a function that sets the subscription validator to return nil for the given peer ids.
// It is used for testing purposes and causes the given peer id to never be penalized for subscribing to invalid topics.
func withValidSubscriptions(peerIds ...peer.ID) func(cfg *scoring.GossipSubAppSpecificScoreRegistryConfig) {
	return func(cfg *scoring.GossipSubAppSpecificScoreRegistryConfig) {
		cfg.Validator.(*mockp2p.SubscriptionValidator).
			On("CheckSubscribedToAllowedTopics", testifymock.AnythingOfType("peer.ID"), testifymock.Anything).
			Return(func(pid peer.ID, _ flow.Role) error {
				for _, peerID := range peerIds {
					if peerID == pid {
						return nil
					}
				}
				return fmt.Errorf("invalid subscriptions")
			}).Maybe()
	}
}

// withUnknownIdentity returns a function that sets the identity provider to return an error for the given peer id.
// It is used for testing purposes, and causes the given peer id to be penalized for not having a staked identity.
func withUnknownIdentity(peer peer.ID) func(cfg *scoring.GossipSubAppSpecificScoreRegistryConfig) {
	return func(cfg *scoring.GossipSubAppSpecificScoreRegistryConfig) {
		cfg.IdProvider.(*mock.IdentityProvider).On("ByPeerID", peer).Return(nil, false).Maybe()
	}
}

// withInvalidSubscriptions returns a function that sets the subscription validator to return an error for the given peer id.
// It is used for testing purposes and causes the given peer id to be penalized for subscribing to invalid topics.
func withInvalidSubscriptions(peer peer.ID) func(cfg *scoring.GossipSubAppSpecificScoreRegistryConfig) {
	return func(cfg *scoring.GossipSubAppSpecificScoreRegistryConfig) {
		cfg.Validator.(*mockp2p.SubscriptionValidator).On("CheckSubscribedToAllowedTopics",
			peer,
			testifymock.Anything).Return(fmt.Errorf("invalid subscriptions")).Maybe()
	}
}

// newGossipSubAppSpecificScoreRegistry creates a new instance of GossipSubAppSpecificScoreRegistry along with its associated
// GossipSubSpamRecordCache and AppSpecificScoreCache. This function is primarily used in testing scenarios to set up a controlled
// environment for evaluating the behavior of the GossipSub scoring mechanism.
//
// The function accepts a variable number of options to configure the GossipSubAppSpecificScoreRegistryConfig, allowing for
// customization of the registry's behavior in tests. These options can modify various aspects of the configuration, such as
// penalty values, identity providers, validators, and caching mechanisms.
//
// Parameters:
// - t *testing.T: The test context, used for asserting the absence of errors during the setup.
// - params p2pconfig.ScoringParameters: The scoring parameters used to configure the registry.
// - initFunction scoring.SpamRecordInitFunc: The function used to initialize the spam records.
// - opts ...func(*scoring.GossipSubAppSpecificScoreRegistryConfig): A variadic set of functions that modify the registry's configuration.
//
// Returns:
// - *scoring.GossipSubAppSpecificScoreRegistry: The configured GossipSub application-specific score registry.
// - *netcache.GossipSubSpamRecordCache: The cache used for storing spam records.
// - *internal.AppSpecificScoreCache: The cache for storing application-specific scores.
//
// This function initializes and configures the scoring registry with default and test-specific settings. It sets up a spam record cache
// and an application-specific score cache with predefined sizes and functionalities. The function also configures the scoring parameters
// with test-specific values, particularly modifying the ScoreTTL value for the purpose of the tests. The creation and configuration of
// the GossipSubAppSpecificScoreRegistry are validated to ensure no errors occur during the process.
func newGossipSubAppSpecificScoreRegistry(t *testing.T,
	params p2pconfig.ScoringParameters,
	initFunction scoring.SpamRecordInitFunc,
	opts ...func(*scoring.GossipSubAppSpecificScoreRegistryConfig)) (*scoring.GossipSubAppSpecificScoreRegistry,
	*netcache.GossipSubSpamRecordCache,
	*internal.AppSpecificScoreCache) {
	cache := netcache.NewGossipSubSpamRecordCache(100,
		unittest.Logger(),
		metrics.NewNoopCollector(),
		initFunction,
		scoring.DefaultDecayFunction(params.ScoringRegistryParameters.SpamRecordCache.Decay))
	appSpecificScoreCache := internal.NewAppSpecificScoreCache(100, unittest.Logger(), metrics.NewNoopCollector())

	validator := mockp2p.NewSubscriptionValidator(t)
	validator.On("Start", testifymock.Anything).Return().Maybe()
	done := make(chan struct{})
	close(done)
	f := func() <-chan struct{} {
		return done
	}
	validator.On("Ready").Return(f()).Maybe()
	validator.On("Done").Return(f()).Maybe()
	cfg := &scoring.GossipSubAppSpecificScoreRegistryConfig{
		Logger:     unittest.Logger(),
		Penalty:    penaltyValueFixtures(),
		IdProvider: mock.NewIdentityProvider(t),
		Validator:  validator,
		AppScoreCacheFactory: func() p2p.GossipSubApplicationSpecificScoreCache {
			return appSpecificScoreCache
		},
		SpamRecordCacheFactory: func() p2p.GossipSubSpamRecordCache {
			return cache
		},
		Parameters:              params.ScoringRegistryParameters.AppSpecificScore,
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		NetworkingType:          network.PrivateNetwork,
		AppSpecificScoreParams:  params.PeerScoring.Protocol.AppSpecificScore,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	reg, err := scoring.NewGossipSubAppSpecificScoreRegistry(cfg)
	require.NoError(t, err, "failed to create GossipSubAppSpecificScoreRegistry")

	return reg, cache, appSpecificScoreCache
}

// penaltyValueFixtures returns a set of penalty values for testing purposes.
// The values are not realistic. The important thing is that they are different from each other. This is to make sure
// that the tests are not passing because of the default values.
func penaltyValueFixtures() p2pconfig.MisbehaviourPenalties {
	return p2pconfig.MisbehaviourPenalties{
		GraftMisbehaviour:              -100,
		PruneMisbehaviour:              -50,
		IHaveMisbehaviour:              -20,
		IWantMisbehaviour:              -10,
		ClusterPrefixedReductionFactor: .5,
		PublishMisbehaviour:            -10,
	}
}

// penaltyValueFixture returns the set penalty of the provided control message type returned from the fixture func penaltyValueFixtures.
func penaltyValueFixture(msgType p2pmsg.ControlMessageType) float64 {
	penaltyValues := penaltyValueFixtures()
	switch msgType {
	case p2pmsg.CtrlMsgGraft:
		return penaltyValues.GraftMisbehaviour
	case p2pmsg.CtrlMsgPrune:
		return penaltyValues.PruneMisbehaviour
	case p2pmsg.CtrlMsgIHave:
		return penaltyValues.IHaveMisbehaviour
	case p2pmsg.CtrlMsgIWant:
		return penaltyValues.IWantMisbehaviour
	case p2pmsg.RpcPublishMessage:
		return penaltyValues.PublishMisbehaviour
	default:
		return penaltyValues.ClusterPrefixedReductionFactor
	}
}
