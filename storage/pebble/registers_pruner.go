package pebble

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
)

const (
	DefaultPruneThreshold      = uint64(100_000)
	DefaultPruneThrottleDelay  = 10 * time.Millisecond
	DefaultPruneTickerInterval = 10 * time.Minute
)

// pruneIntervalRatio represents an additional percentage of pruneThreshold which is used to calculate pruneInterval
const pruneIntervalRatio = 0.1

// pruneInterval is a helper function which calculates interval for pruner
func pruneInterval(threshold uint64) uint64 {
	return threshold + uint64(float64(threshold)*pruneIntervalRatio)
}

type RegisterPruner struct {
	component.Component
	componentManager *component.ComponentManager

	logger   zerolog.Logger
	register storage.RegisterIndex

	// pruningInterval is a number of pruned blocks in the db, above which pruning should be triggered
	pruneInterval uint64
	// threshold defines the number of blocks below the latestHeight to keep
	pruneThreshold uint64
	// pruneThrottleDelay controls a small pause between batches of registers inspected and pruned
	pruneThrottleDelay time.Duration
	// pruneTickerInterval defines how frequently pruning can be performed
	pruneTickerInterval time.Duration
}

type PrunerOption func(*RegisterPruner)

// WithPruneThreshold is used to configure the pruner with a custom threshold.
func WithPruneThreshold(threshold uint64) PrunerOption {
	return func(p *RegisterPruner) {
		p.pruneThreshold = threshold
		p.pruneInterval = pruneInterval(threshold)
	}
}

// WithPruneThrottleDelay is used to configure the pruner with a custom
// throttle delay.
func WithPruneThrottleDelay(throttleDelay time.Duration) PrunerOption {
	return func(p *RegisterPruner) {
		p.pruneThrottleDelay = throttleDelay
	}
}

// WithPruneTickerInterval is used to configure the pruner with a custom
// ticker interval.
func WithPruneTickerInterval(interval time.Duration) PrunerOption {
	return func(p *RegisterPruner) {
		p.pruneTickerInterval = interval
	}
}

func NewRegisterPruner(
	logger zerolog.Logger,
	register storage.RegisterIndex,
	opts ...PrunerOption,
) (*RegisterPruner, error) {
	pruner := &RegisterPruner{
		logger:              logger.With().Str("component", "register_pruner").Logger(),
		register:            register,
		pruneInterval:       pruneInterval(DefaultPruneThreshold),
		pruneThreshold:      DefaultPruneThreshold,
		pruneThrottleDelay:  DefaultPruneThrottleDelay,
		pruneTickerInterval: DefaultPruneTickerInterval,
	}

	pruner.componentManager = component.NewComponentManagerBuilder().
		AddWorker(pruner.loop).
		Build()
	pruner.Component = pruner.componentManager

	for _, opt := range opts {
		opt(pruner)
	}

	return pruner, nil
}

// loop is the main worker for the Pruner, responsible for triggering
// pruning operations at regular intervals. It monitors the heights
// of registered height recorders and checks if pruning is necessary.
func (p *RegisterPruner) loop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()
	ticker := time.NewTicker(p.pruneTickerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.checkPrune(ctx)
		}
	}
}

// checkPrune checks if pruning should be performed based on the height range
// and triggers the pruning operation if necessary.
//
// Parameters:
//   - ctx: The context for managing the pruning operation.
func (p *RegisterPruner) checkPrune(ctx irrecoverable.SignalerContext) {
	firstHeight, err := p.register.FirstStoredHeight()
	if err != nil {
		ctx.Throw(fmt.Errorf("failed to get first height from register storage: %w", err))
	}

	latestHeight, err := p.register.LatestStoredHeight()
	if err != nil {
		ctx.Throw(fmt.Errorf("failed to get latest height from register storage: %w", err))
	}

	pruneHeight := latestHeight - p.pruneThreshold

	if pruneHeight-firstHeight > p.pruneInterval {
		p.logger.Info().Uint64("prune_height", pruneHeight).Msg("pruning storage")

		err := p.register.PruneUpToHeight(pruneHeight)
		if err != nil {
			ctx.Throw(fmt.Errorf("failed to prune: %w", err))
		}

		// update first indexed height
		err = p.register.UpdateFirstStoredHeight(pruneHeight)
		if err != nil {
			ctx.Throw(fmt.Errorf("failed to update first height for register storage: %w", err))
		}
	}
}
