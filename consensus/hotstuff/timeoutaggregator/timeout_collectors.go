package timeoutaggregator

import (
	"fmt"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/module/mempool"
)

// NewCollectorFactoryMethod is a factory method to generate a TimeoutCollector for concrete view
type NewCollectorFactoryMethod = func(view uint64) (hotstuff.TimeoutCollector, error)

// TimeoutCollectors implements management of multiple timeout collectors indexed by view.
// Implements hotstuff.TimeoutCollectors interface. Creating a TimeoutCollector for a
// particular view is lazy (instances are created on demand).
// This structure is concurrently safe.
// TODO: once VoteCollectors gets updated to stop managing worker pool we can merge VoteCollectors and TimeoutCollectors using generics
type TimeoutCollectors struct {
	log                zerolog.Logger
	lock               sync.RWMutex
	lowestRetainedView uint64                               // lowest view, for which we still retain a TimeoutCollector and process timeouts
	collectors         map[uint64]hotstuff.TimeoutCollector // view -> TimeoutCollector
	createCollector    NewCollectorFactoryMethod            // factory method for creating collectors
}

var _ hotstuff.TimeoutCollectors = (*TimeoutCollectors)(nil)

// GetOrCreateCollector retrieves the hotstuff.TimeoutCollector for the specified
// view or creates one if none exists.
//  -  (collector, true, nil) if no collector can be found by the view, and a new collector was created.
//  -  (collector, false, nil) if the collector can be found by the view
//  -  (nil, false, error) if running into any exception creating the timeout collector state machine
// Expected error returns during normal operations:
//  * mempool.DecreasingPruningHeightError - in case view is lower than lowestRetainedView
func (t *TimeoutCollectors) GetOrCreateCollector(view uint64) (hotstuff.TimeoutCollector, bool, error) {
	cachedCollector, hasCachedCollector, err := t.getCollector(view)
	if err != nil {
		return nil, false, err
	}

	if hasCachedCollector {
		return cachedCollector, false, nil
	}

	collector, err := t.createCollector(view)
	if err != nil {
		return nil, false, fmt.Errorf("could not create timeout collector for view %d: %w", view, err)
	}

	// Initial check showed that there was no collector. However, it's possible that after the
	// initial check but before acquiring the lock to add the newly-created collector, another
	// goroutine already added the needed collector. Hence, check again after acquiring the lock:
	t.lock.Lock()
	defer t.lock.Unlock()

	clr, found := t.collectors[view]
	if found {
		return clr, false, nil
	}

	t.collectors[view] = collector
	return collector, true, nil
}

// getCollector retrieves hotstuff.TimeoutCollector from local cache in concurrent safe way.
// Performs check for lowestRetainedView.
// Expected error returns during normal operations:
//  * mempool.DecreasingPruningHeightError - in case view is lower than lowestRetainedView
func (t *TimeoutCollectors) getCollector(view uint64) (hotstuff.TimeoutCollector, bool, error) {
	t.lock.RLock()
	defer t.lock.RUnlock()
	if view < t.lowestRetainedView {
		return nil, false, mempool.NewDecreasingPruningHeightErrorf("cannot retrieve collector for pruned view %d (lowest retained view %d)", view, t.lowestRetainedView)
	}

	clr, found := t.collectors[view]

	return clr, found, nil
}

// PruneUpToView prunes the timeout collectors with views _below_ the given value, i.e.
// we only retain and process whose view is equal or larger than `lowestRetainedView`.
// If `lowestRetainedView` is smaller than the previous value, the previous value is
// kept and the method call is a NoOp.
func (t *TimeoutCollectors) PruneUpToView(lowestRetainedView uint64) {
	t.lock.Lock()
	defer t.lock.Unlock()
	if t.lowestRetainedView >= lowestRetainedView {
		return
	}
	if len(t.collectors) == 0 {
		t.lowestRetainedView = lowestRetainedView
		return
	}

	sizeBefore := len(t.collectors)

	// to optimize the pruning of large view-ranges, we compare:
	//  * the number of views for which we have collectors: len(t.collectors)
	//  * the number of views that need to be pruned: view-t.lowestRetainedView
	// We iterate over the dimension which is smaller.
	if uint64(len(t.collectors)) < lowestRetainedView-t.lowestRetainedView {
		for w := range t.collectors {
			if w < lowestRetainedView {
				delete(t.collectors, w)
			}
		}
	} else {
		for w := t.lowestRetainedView; w < lowestRetainedView; w++ {
			delete(t.collectors, w)
		}
	}
	from := t.lowestRetainedView
	t.lowestRetainedView = lowestRetainedView

	t.log.Debug().
		Uint64("prior_lowest_retained_view", from).
		Uint64("lowest_retained_view", lowestRetainedView).
		Int("prior_size", sizeBefore).
		Int("size", len(t.collectors)).
		Msgf("pruned timeout collectors")
}
