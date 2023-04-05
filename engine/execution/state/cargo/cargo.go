package cargo

import (
	"github.com/onflow/flow-go/model/flow"
)

type Cargo struct {
	blockQueue *FinalizedBlockQueue
	views      *Views
}

func NewCargo(
	storage Storage,
	blockQueueCapacity int,
	genesis *flow.Header,
) (*Cargo, error) {
	views, err := NewViews(storage) // TODO pass startBlockParent to Views as well for validation
	if err != nil {
		return nil, err
	}
	return &Cargo{
		blockQueue: NewFinalizedBlockQueue(blockQueueCapacity, genesis),
		views:      views,
	}, nil
}

func (c *Cargo) Reader(header *flow.Header) *Reader {
	return NewReader(header, c.views)
}

func (c *Cargo) BlockFinalized(new *flow.Header) error {
	// first enqueue the header
	// if we reach a capacity that we could not enqueu blocks and they stay uncommitable
	// then here we are returning an error
	if err := c.blockQueue.Enqueue(new); err != nil {
		return err
	}

	// then trigger sync until not commitable
	blockID, header := c.blockQueue.Peak()
	for found, err := c.views.Commit(blockID, header); found; {
		if err != nil {
			return err
		}
		c.blockQueue.Dequeue()
		blockID, header = c.blockQueue.Peak()
	}

	return nil
}

func (c *Cargo) Update(header *flow.Header, delta map[flow.RegisterID]flow.RegisterValue) error {
	c.views.Set(header, delta)
	// we don't trigger actions here just collect in the next block finalized we deal with the gap
	return nil
}
