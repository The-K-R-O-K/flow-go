package migrations

import (
	"github.com/onflow/atree"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/stdlib"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/model/flow"
)

type RegistersMigration func(registersByAccount *registers.ByAccount) error

var AllStorageMapDomains = []string{
	common.PathDomainStorage.Identifier(),
	common.PathDomainPrivate.Identifier(),
	common.PathDomainPublic.Identifier(),
	runtime.StorageDomainContract,
	stdlib.InboxStorageDomain,
	stdlib.CapabilityControllerStorageDomain,
	stdlib.PathCapabilityStorageDomain,
	stdlib.AccountCapabilityStorageDomain,
}

var allStorageMapDomainsSet = map[string]struct{}{}

func init() {
	for _, domain := range AllStorageMapDomains {
		allStorageMapDomainsSet[domain] = struct{}{}
	}
}

func getSlabIDsFromRegisters(registers registers.Registers) ([]atree.SlabID, error) {
	storageIDs := make([]atree.SlabID, 0, registers.Count())

	err := registers.ForEach(func(owner string, key string, value []byte) error {

		if !flow.IsSlabIndexKey(key) {
			return nil
		}

		slabID := atree.NewSlabID(
			atree.Address([]byte(owner)),
			atree.SlabIndex([]byte(key[1:])),
		)

		storageIDs = append(storageIDs, slabID)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return storageIDs, nil
}

func loadAtreeSlabsInStorage(
	storage *runtime.Storage,
	registers registers.Registers,
	nWorkers int,
) error {

	storageIDs, err := getSlabIDsFromRegisters(registers)
	if err != nil {
		return err
	}

	return storage.PersistentSlabStorage.BatchPreload(storageIDs, nWorkers)
}

func checkStorageHealth(
	address common.Address,
	storage *runtime.Storage,
	registers registers.Registers,
	nWorkers int,
) error {

	err := loadAtreeSlabsInStorage(storage, registers, nWorkers)
	if err != nil {
		return err
	}

	for _, domain := range AllStorageMapDomains {
		_ = storage.GetStorageMap(address, domain, false)
	}

	return storage.CheckHealth()
}
