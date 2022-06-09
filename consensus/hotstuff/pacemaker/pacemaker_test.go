package pacemaker

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/pacemaker/timeout"
	"github.com/onflow/flow-go/model/flow"
)

const (
	startRepTimeout        float64 = 400.0 // Milliseconds
	minRepTimeout          float64 = 100.0 // Milliseconds
	voteTimeoutFraction    float64 = 0.5   // multiplicative factor
	multiplicativeIncrease float64 = 1.5   // multiplicative factor
	multiplicativeDecrease float64 = 0.85  // multiplicative factor
)

func expectedTimerInfo(view uint64, mode model.TimeoutMode) interface{} {
	return mock.MatchedBy(
		func(timerInfo *model.TimerInfo) bool {
			return timerInfo.View == view && timerInfo.Mode == mode
		})
}

func TestActivePaceMaker(t *testing.T) {
	suite.Run(t, new(ActivePaceMakerTestSuite))
}

type ActivePaceMakerTestSuite struct {
	suite.Suite

	livenessData *hotstuff.LivenessData
	notifier     *mocks.Consumer
	persist      *mocks.Persister
	paceMaker    *ActivePaceMaker
}

func (s *ActivePaceMakerTestSuite) SetupTest() {
	s.notifier = &mocks.Consumer{}
	s.persist = &mocks.Persister{}

	tc, err := timeout.NewConfig(
		time.Duration(startRepTimeout*1e6),
		time.Duration(minRepTimeout*1e6),
		voteTimeoutFraction,
		multiplicativeIncrease,
		multiplicativeDecrease,
		0)
	require.NoError(s.T(), err)

	s.livenessData = &hotstuff.LivenessData{
		CurrentView: 3,
		LastViewTC:  nil,
		NewestQC:    helper.MakeQC(helper.WithQCView(2)),
	}

	s.persist.On("GetLivenessData").Return(s.livenessData, nil).Once()

	s.paceMaker, err = New(timeout.NewController(tc), s.notifier, s.persist)
	require.NoError(s.T(), err)

	s.notifier.On("OnStartingTimeout", expectedTimerInfo(s.livenessData.CurrentView, model.ReplicaTimeout)).Return().Once()

	s.paceMaker.Start()
}

func QC(view uint64) *flow.QuorumCertificate {
	return &flow.QuorumCertificate{View: view}
}

func LivenessData(qc *flow.QuorumCertificate) *hotstuff.LivenessData {
	return &hotstuff.LivenessData{
		CurrentView: qc.View + 1,
		LastViewTC:  nil,
		NewestQC:    qc,
	}
}

// TestProcessQC_SkipIncreaseViewThroughQC tests that ActivePaceMaker increases view when receiving QC,
// if applicable, by skipping views
func (s *ActivePaceMakerTestSuite) TestProcessQC_SkipIncreaseViewThroughQC() {
	qc := QC(s.livenessData.CurrentView)
	s.persist.On("PutLivenessData", LivenessData(qc)).Return(nil).Once()
	s.notifier.On("OnStartingTimeout", expectedTimerInfo(4, model.ReplicaTimeout)).Return().Once()
	s.notifier.On("OnQcTriggeredViewChange", qc, uint64(4)).Return().Once()
	nve, err := s.paceMaker.ProcessQC(qc)
	require.NoError(s.T(), err)
	s.notifier.AssertExpectations(s.T())
	require.Equal(s.T(), qc.View+1, s.paceMaker.CurView())
	require.True(s.T(), nve.View == qc.View+1)
	require.Equal(s.T(), qc, s.paceMaker.NewestQC())
	require.Nil(s.T(), s.paceMaker.LastViewTC())

	qc = QC(12)
	s.persist.On("PutLivenessData", LivenessData(qc)).Return(nil).Once()
	s.notifier.On("OnStartingTimeout", expectedTimerInfo(13, model.ReplicaTimeout)).Return().Once()
	s.notifier.On("OnQcTriggeredViewChange", qc, uint64(13)).Return().Once()
	nve, err = s.paceMaker.ProcessQC(qc)
	require.NoError(s.T(), err)
	require.True(s.T(), nve.View == qc.View+1)
	require.Equal(s.T(), qc, s.paceMaker.NewestQC())
	require.Nil(s.T(), s.paceMaker.LastViewTC())

	s.notifier.AssertExpectations(s.T())
	require.Equal(s.T(), qc.View+1, s.paceMaker.CurView())
}

// TestProcessTC_SkipIncreaseViewThroughTC tests that ActivePaceMaker increases view when receiving TC,
// if applicable, by skipping views
func (s *ActivePaceMakerTestSuite) TestProcessTC_SkipIncreaseViewThroughTC() {
	tc := helper.MakeTC(helper.WithTCView(s.livenessData.CurrentView),
		helper.WithTCNewestQC(s.livenessData.NewestQC))
	expectedLivenessData := &hotstuff.LivenessData{
		CurrentView: tc.View + 1,
		LastViewTC:  tc,
		NewestQC:    tc.NewestQC,
	}
	s.persist.On("PutLivenessData", expectedLivenessData).Return(nil).Once()
	s.notifier.On("OnStartingTimeout", expectedTimerInfo(tc.View+1, model.ReplicaTimeout)).Return().Once()
	s.notifier.On("OnTcTriggeredViewChange", tc, tc.View+1).Return().Once()
	nve, err := s.paceMaker.ProcessTC(tc)
	require.NoError(s.T(), err)
	s.notifier.AssertExpectations(s.T())
	require.Equal(s.T(), tc.View+1, s.paceMaker.CurView())
	require.True(s.T(), nve.View == tc.View+1)
	require.Equal(s.T(), tc, s.paceMaker.LastViewTC())

	// skip 10 views
	tc = helper.MakeTC(helper.WithTCView(tc.View+10),
		helper.WithTCNewestQC(s.livenessData.NewestQC),
		helper.WithTCNewestQC(QC(s.livenessData.CurrentView)))
	expectedLivenessData = &hotstuff.LivenessData{
		CurrentView: tc.View + 1,
		LastViewTC:  tc,
		NewestQC:    tc.NewestQC,
	}
	s.persist.On("PutLivenessData", expectedLivenessData).Return(nil).Once()
	s.notifier.On("OnStartingTimeout", expectedTimerInfo(tc.View+1, model.ReplicaTimeout)).Return().Once()
	s.notifier.On("OnTcTriggeredViewChange", tc, tc.View+1).Return().Once()
	nve, err = s.paceMaker.ProcessTC(tc)
	require.NoError(s.T(), err)
	require.True(s.T(), nve.View == tc.View+1)
	require.Equal(s.T(), tc, s.paceMaker.LastViewTC())
	require.Equal(s.T(), tc.NewestQC, s.paceMaker.NewestQC())

	s.notifier.AssertExpectations(s.T())
	require.Equal(s.T(), tc.View+1, s.paceMaker.CurView())
}

// TestProcessQC_PersistException tests that ActivePaceMaker propagates exception
// when processing QC
func (s *ActivePaceMakerTestSuite) TestProcessQC_PersistException() {
	exception := errors.New("persist-exception")
	qc := QC(s.livenessData.CurrentView)
	s.persist.On("PutLivenessData", mock.Anything).Return(exception).Once()
	nve, err := s.paceMaker.ProcessQC(qc)
	require.Nil(s.T(), nve)
	require.ErrorIs(s.T(), err, exception)
}

// TestProcessTC_PersistException tests that ActivePaceMaker propagates exception
// when processing TC
func (s *ActivePaceMakerTestSuite) TestProcessTC_PersistException() {
	exception := errors.New("persist-exception")
	tc := helper.MakeTC(helper.WithTCView(s.livenessData.CurrentView))
	s.persist.On("PutLivenessData", mock.Anything).Return(exception).Once()
	nve, err := s.paceMaker.ProcessTC(tc)
	require.Nil(s.T(), nve)
	require.ErrorIs(s.T(), err, exception)
}

// TestProcessQC_IgnoreOldQC tests that ActivePaceMaker ignores old QCs
func (s *ActivePaceMakerTestSuite) TestProcessQC_IgnoreOldQC() {
	nve, err := s.paceMaker.ProcessQC(QC(2))
	require.NoError(s.T(), err)
	require.Nil(s.T(), nve)
	s.notifier.AssertExpectations(s.T())
	require.Equal(s.T(), uint64(3), s.paceMaker.CurView())
}

// TestOnPartialTC_TriggersTimeout tests that ActivePaceMaker ignores partial TCs and triggers
// timeout for active view
func (s *ActivePaceMakerTestSuite) TestOnPartialTC_TriggersTimeout() {
	// report previously known view
	s.paceMaker.OnPartialTC(s.livenessData.CurrentView - 1)
	// this shouldn't trigger a timeout
	select {
	case <-s.paceMaker.TimeoutChannel():
		s.Fail("triggered timeout channel")
	case <-time.After(time.Duration(startRepTimeout/2) * time.Millisecond):
	}

	qc := helper.MakeQC(helper.WithQCView(s.livenessData.CurrentView + 1))

	s.persist.On("PutLivenessData", mock.Anything).Return(nil).Once()
	s.notifier.On("OnStartingTimeout", expectedTimerInfo(qc.View+1, model.ReplicaTimeout)).Return().Once()
	s.notifier.On("OnQcTriggeredViewChange", qc, qc.View+1).Return().Once()
	nve, err := s.paceMaker.ProcessQC(qc)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), nve)

	// reporting partial TC for current view should result in closing of timeout channel
	s.paceMaker.OnPartialTC(s.paceMaker.CurView())

	select {
	case <-s.paceMaker.TimeoutChannel():
	case <-time.After(time.Duration(startRepTimeout/2) * time.Millisecond):
		s.Fail("Timeout has to be triggered earlier than configured")
	}
}
