package migrations

import (
	_ "embed"

	"github.com/onflow/cadence/migrations/capcons"
	"github.com/onflow/cadence/migrations/statictypes"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

func NewCadence1InterfaceStaticTypeConverter(chainID flow.ChainID) statictypes.InterfaceTypeConverterFunc {
	systemContracts := systemcontracts.SystemContractsForChain(chainID)

	oldFungibleTokenResolverType, newFungibleTokenResolverType := fungibleTokenResolverRule(systemContracts)

	rules := StaticTypeMigrationRules{
		oldFungibleTokenResolverType.ID(): newFungibleTokenResolverType,
	}

	return NewStaticTypeMigrator[*interpreter.InterfaceStaticType](rules)
}

func NewCadence1CompositeStaticTypeConverter(chainID flow.ChainID) statictypes.CompositeTypeConverterFunc {

	systemContracts := systemcontracts.SystemContractsForChain(chainID)

	oldFungibleTokenVaultCompositeType, newFungibleTokenVaultType := fungibleTokenVaultRule(systemContracts)
	oldNonFungibleTokenNFTCompositeType, newNonFungibleTokenNFTType := nonFungibleTokenNFTRule(systemContracts)

	rules := StaticTypeMigrationRules{
		oldFungibleTokenVaultCompositeType.ID():  newFungibleTokenVaultType,
		oldNonFungibleTokenNFTCompositeType.ID(): newNonFungibleTokenNFTType,
	}

	return NewStaticTypeMigrator[*interpreter.CompositeStaticType](rules)
}

func nonFungibleTokenNFTRule(
	systemContracts *systemcontracts.SystemContracts,
) (
	*interpreter.CompositeStaticType,
	*interpreter.IntersectionStaticType,
) {
	contract := systemContracts.NonFungibleToken

	qualifiedIdentifier := contract.Name + ".NFT"

	location := common.AddressLocation{
		Address: common.Address(contract.Address),
		Name:    contract.Name,
	}

	nftTypeID := location.TypeID(nil, qualifiedIdentifier)

	oldType := &interpreter.CompositeStaticType{
		Location:            location,
		QualifiedIdentifier: qualifiedIdentifier,
		TypeID:              nftTypeID,
	}

	newType := &interpreter.IntersectionStaticType{
		Types: []*interpreter.InterfaceStaticType{
			{
				Location:            location,
				QualifiedIdentifier: qualifiedIdentifier,
				TypeID:              nftTypeID,
			},
		},
	}

	return oldType, newType
}

func fungibleTokenVaultRule(
	systemContracts *systemcontracts.SystemContracts,
) (
	*interpreter.CompositeStaticType,
	*interpreter.IntersectionStaticType,
) {
	contract := systemContracts.FungibleToken

	qualifiedIdentifier := contract.Name + ".Vault"

	location := common.AddressLocation{
		Address: common.Address(contract.Address),
		Name:    contract.Name,
	}

	vaultTypeID := location.TypeID(nil, qualifiedIdentifier)

	oldType := &interpreter.CompositeStaticType{
		Location:            location,
		QualifiedIdentifier: qualifiedIdentifier,
		TypeID:              vaultTypeID,
	}

	newType := &interpreter.IntersectionStaticType{
		Types: []*interpreter.InterfaceStaticType{
			{
				Location:            location,
				QualifiedIdentifier: qualifiedIdentifier,
				TypeID:              vaultTypeID,
			},
		},
	}

	return oldType, newType
}

func fungibleTokenResolverRule(
	systemContracts *systemcontracts.SystemContracts,
) (
	*interpreter.InterfaceStaticType,
	*interpreter.InterfaceStaticType,
) {
	oldContract := systemContracts.MetadataViews
	newContract := systemContracts.ViewResolver

	oldLocation := common.AddressLocation{
		Address: common.Address(oldContract.Address),
		Name:    oldContract.Name,
	}

	newLocation := common.AddressLocation{
		Address: common.Address(newContract.Address),
		Name:    newContract.Name,
	}

	oldQualifiedIdentifier := oldContract.Name + ".Resolver"
	newQualifiedIdentifier := newContract.Name + ".Resolver"

	oldType := &interpreter.InterfaceStaticType{
		Location:            oldLocation,
		QualifiedIdentifier: oldQualifiedIdentifier,
		TypeID:              oldLocation.TypeID(nil, oldQualifiedIdentifier),
	}

	newType := &interpreter.InterfaceStaticType{
		Location:            newLocation,
		QualifiedIdentifier: newQualifiedIdentifier,
		TypeID:              newLocation.TypeID(nil, newQualifiedIdentifier),
	}

	return oldType, newType
}

func NewCadence1ValueMigrations(
	log zerolog.Logger,
	rwf reporters.ReportWriterFactory,
	nWorker int,
	chainID flow.ChainID,
) (migrations []AccountBasedMigration) {

	// Populated by CadenceLinkValueMigrator,
	// used by CadenceCapabilityValueMigrator
	capabilityIDs := &capcons.CapabilityIDMapping{}

	return []AccountBasedMigration{
		NewCadence1ValueMigrator(
			rwf,
			NewCadence1CompositeStaticTypeConverter(chainID),
			NewCadence1InterfaceStaticTypeConverter(chainID),
		),
		NewCadence1LinkValueMigrator(rwf, capabilityIDs),
		NewCadence1CapabilityValueMigrator(rwf, capabilityIDs),
	}
}

func NewCadence1Migrations(
	log zerolog.Logger,
	rwf reporters.ReportWriterFactory,
	nWorker int,
	chainID flow.ChainID,
	evmContractChange EVMContractChange,
	stagedContracts []StagedContract,
) []ledger.Migration {

	return []ledger.Migration{

		NewAccountBasedMigration(
			log,
			nWorker,
			[]AccountBasedMigration{
				NewSystemContactsMigration(
					chainID,
					SystemContractChangesOptions{
						EVM: evmContractChange,
					},
				),
			},
		),

		NewBurnerDeploymentMigration(chainID, log),

		NewAccountBasedMigration(
			log,
			nWorker,
			append(
				[]AccountBasedMigration{NewStagedContractsMigration(stagedContracts)},
				NewCadence1ValueMigrations(log, rwf, nWorker, chainID)...,
			),
		),
	}
}
