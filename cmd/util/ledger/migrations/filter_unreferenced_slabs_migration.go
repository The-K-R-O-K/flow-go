package migrations

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/onflow/atree"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/interpreter"
	"github.com/onflow/cadence/runtime"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

func registerFromSlabID(slabID atree.SlabID) (owner, key string) {
	var address [8]byte
	binary.BigEndian.PutUint64(address[:], slabID.AddressAsUint64())

	index := slabID.Index()

	owner = string(address[:])

	var sb strings.Builder
	sb.WriteByte(flow.SlabIndexPrefix)
	sb.Write(index[:])
	key = sb.String()

	return owner, key
}

type FilterUnreferencedSlabsMigration struct {
	log              zerolog.Logger
	rw               reporters.ReportWriter
	outputDir        string
	mutex            sync.Mutex
	filteredPayloads []*ledger.Payload
	payloadsFile     string
	nWorkers         int
}

var _ AccountBasedMigration = &FilterUnreferencedSlabsMigration{}

const filterUnreferencedSlabsName = "filter-unreferenced-slabs"

func NewFilterUnreferencedSlabsMigration(
	outputDir string,
	rwf reporters.ReportWriterFactory,
) *FilterUnreferencedSlabsMigration {
	return &FilterUnreferencedSlabsMigration{
		outputDir:        outputDir,
		rw:               rwf.ReportWriter(filterUnreferencedSlabsName),
		filteredPayloads: make([]*ledger.Payload, 0, 50_000),
	}
}

func (m *FilterUnreferencedSlabsMigration) InitMigration(
	log zerolog.Logger,
	_ *registers.ByAccount,
	nWorkers int,
) error {
	m.log = log.
		With().
		Str("migration", filterUnreferencedSlabsName).
		Logger()

	m.nWorkers = nWorkers

	return nil
}

func (m *FilterUnreferencedSlabsMigration) MigrateAccount(
	_ context.Context,
	address common.Address,
	accountRegisters *registers.AccountRegisters,
) error {

	storage := runtime.NewStorage(
		registers.ReadOnlyLedger{
			Registers: accountRegisters,
		},
		nil,
		runtime.StorageConfig{},
	)

	inter, err := interpreter.NewInterpreter(
		nil,
		nil,
		&interpreter.Config{
			Storage: storage,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create interpreter: %w", err)
	}

	err = util.CheckStorageHealth(inter, address, storage, accountRegisters, common.AllStorageDomains, m.nWorkers)
	if err == nil {
		return nil
	}

	// The storage health check failed.
	// This can happen if there are unreferenced root slabs.
	// In this case, we filter out the unreferenced root slabs and all slabs they reference from the payloads.

	var unreferencedRootSlabsErr runtime.UnreferencedRootSlabsError
	if !errors.As(err, &unreferencedRootSlabsErr) {
		return fmt.Errorf("storage health check failed: %w", err)
	}

	// Create a set of unreferenced slabs: root slabs, and all slabs they reference.

	unreferencedSlabIDs := map[atree.SlabID]struct{}{}
	for _, rootSlabID := range unreferencedRootSlabsErr.UnreferencedRootSlabIDs {
		unreferencedSlabIDs[rootSlabID] = struct{}{}

		childReferences, _, err := storage.GetAllChildReferences(rootSlabID)
		if err != nil {
			return fmt.Errorf(
				"failed to get all child references for root slab %s: %w",
				rootSlabID,
				err,
			)
		}

		for _, childSlabID := range childReferences {
			unreferencedSlabIDs[childSlabID] = struct{}{}
		}
	}

	// Filter out unreferenced slabs.

	filteredPayloads := make([]*ledger.Payload, 0, len(unreferencedSlabIDs))

	m.log.Warn().
		Str("account", address.HexWithPrefix()).
		Msgf("filtering %d unreferenced slabs", len(unreferencedSlabIDs))

	var slabIDs []atree.SlabID
	for storageID := range unreferencedSlabIDs {
		slabIDs = append(slabIDs, storageID)
	}
	sort.Slice(
		slabIDs,
		func(i, j int) bool {
			a := slabIDs[i]
			b := slabIDs[j]
			return a.Compare(b) < 0
		},
	)

	for _, slabID := range slabIDs {
		owner, key := registerFromSlabID(slabID)

		value, err := accountRegisters.Get(owner, key)
		if err != nil {
			return fmt.Errorf(
				"failed to get register for slab %x/%x: %w",
				owner,
				slabID.Index(),
				err,
			)
		}

		err = accountRegisters.Set(owner, key, nil)
		if err != nil {
			return fmt.Errorf(
				"failed to set register for slab %x/%x: %w",
				owner,
				slabID.Index(),
				err,
			)
		}

		ledgerKey := convert.RegisterIDToLedgerKey(flow.RegisterID{
			Owner: owner,
			Key:   key,
		})
		payload := ledger.NewPayload(ledgerKey, value)
		filteredPayloads = append(filteredPayloads, payload)
	}

	m.rw.Write(unreferencedSlabs{
		Account:      address.Hex(),
		PayloadCount: len(filteredPayloads),
	})

	m.mergeFilteredPayloads(filteredPayloads)

	// Do NOT report the health check error here.
	// The health check error is only reported if it is not due to unreferenced slabs.
	// If it is due to unreferenced slabs, we filter them out and continue.

	return nil
}

func (m *FilterUnreferencedSlabsMigration) mergeFilteredPayloads(payloads []*ledger.Payload) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.filteredPayloads = append(m.filteredPayloads, payloads...)
}

func (m *FilterUnreferencedSlabsMigration) Close() error {
	// close the report writer so it flushes to file
	m.rw.Close()

	err := m.writeFilteredPayloads()
	if err != nil {
		return fmt.Errorf("failed to write filtered payloads to file: %w", err)
	}

	return nil
}

func (m *FilterUnreferencedSlabsMigration) writeFilteredPayloads() error {

	m.payloadsFile = path.Join(
		m.outputDir,
		fmt.Sprintf("filtered_%d.payloads", int32(time.Now().Unix())),
	)

	writtenPayloadCount, err := util.CreatePayloadFile(
		m.log,
		m.payloadsFile,
		m.filteredPayloads,
		nil,
		true,
	)

	if err != nil {
		return fmt.Errorf("failed to write all filtered payloads to file: %w", err)
	}

	if writtenPayloadCount != len(m.filteredPayloads) {
		return fmt.Errorf(
			"failed to write all filtered payloads to file: expected %d, got %d",
			len(m.filteredPayloads),
			writtenPayloadCount,
		)
	}

	return nil
}

type unreferencedSlabs struct {
	Account      string `json:"account"`
	PayloadCount int    `json:"payload_count"`
}
