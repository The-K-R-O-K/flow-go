package pubsub

import (
	"sync"
	"time"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// Distributor distributes notifications to a list of subscribers (event consumers).
//
// It allows thread-safe subscription of multiple consumers to events.
type Distributor struct {
	subscribers []hotstuff.Consumer
	lock        sync.RWMutex
}

var _ hotstuff.Consumer = (*Distributor)(nil)

func (p *Distributor) OnEventProcessed() {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnEventProcessed()
	}
}

func NewDistributor() *Distributor {
	return &Distributor{}
}

// AddConsumer adds an a event consumer to the Distributor
func (p *Distributor) AddConsumer(consumer hotstuff.Consumer) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.subscribers = append(p.subscribers, consumer)
}

func (p *Distributor) OnStart(currentView uint64) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnStart(currentView)
	}
}

func (p *Distributor) OnReceiveProposal(currentView uint64, proposal *model.Proposal) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnReceiveProposal(currentView, proposal)
	}
}

func (p *Distributor) OnReceiveQc(currentView uint64, qc *flow.QuorumCertificate) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnReceiveQc(currentView, qc)
	}
}

func (p *Distributor) OnReceiveTc(currentView uint64, tc *flow.TimeoutCertificate) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnReceiveTc(currentView, tc)
	}
}

func (p *Distributor) OnPartialTc(currentView uint64, partialTc *hotstuff.PartialTcCreated) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnPartialTc(currentView, partialTc)
	}
}

func (p *Distributor) OnLocalTimeout(currentView uint64) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnLocalTimeout(currentView)
	}
}

func (p *Distributor) OnViewChange(oldView, newView uint64) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnViewChange(oldView, newView)
	}
}

func (p *Distributor) OnQcTriggeredViewChange(qc *flow.QuorumCertificate, newView uint64) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnQcTriggeredViewChange(qc, newView)
	}
}

func (p *Distributor) OnTcTriggeredViewChange(tc *flow.TimeoutCertificate, newView uint64) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnTcTriggeredViewChange(tc, newView)
	}
}

func (p *Distributor) OnStartingTimeout(timerInfo model.TimerInfo) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnStartingTimeout(timerInfo)
	}
}

func (p *Distributor) OnBlockIncorporated(block *model.Block) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnBlockIncorporated(block)
	}
}

func (p *Distributor) OnFinalizedBlock(block *model.Block) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnFinalizedBlock(block)
	}
}

func (p *Distributor) OnDoubleProposeDetected(block1, block2 *model.Block) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnDoubleProposeDetected(block1, block2)
	}
}

func (p *Distributor) OnDoubleVotingDetected(vote1, vote2 *model.Vote) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnDoubleVotingDetected(vote1, vote2)
	}
}

func (p *Distributor) OnInvalidVoteDetected(vote *model.Vote) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnInvalidVoteDetected(vote)
	}
}

func (p *Distributor) OnVoteForInvalidBlockDetected(vote *model.Vote, invalidProposal *model.Proposal) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnVoteForInvalidBlockDetected(vote, invalidProposal)
	}
}

func (p *Distributor) OnDoubleTimeoutDetected(timeout *model.TimeoutObject, altTimeout *model.TimeoutObject) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnDoubleTimeoutDetected(timeout, altTimeout)
	}
}

func (p *Distributor) OnInvalidTimeoutDetected(timeout *model.TimeoutObject) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, subscriber := range p.subscribers {
		subscriber.OnInvalidTimeoutDetected(timeout)
	}
}

func (p *Distributor) OnOwnVote(blockID flow.Identifier, view uint64, sigData []byte, recipientID flow.Identifier) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, s := range p.subscribers {
		s.OnOwnVote(blockID, view, sigData, recipientID)
	}
}

func (p *Distributor) OnOwnTimeout(timeout *model.TimeoutObject) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, s := range p.subscribers {
		s.OnOwnTimeout(timeout)
	}
}

func (p *Distributor) OnOwnProposal(proposal *flow.Header, targetPublicationTime time.Time) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	for _, s := range p.subscribers {
		s.OnOwnProposal(proposal, targetPublicationTime)
	}
}
