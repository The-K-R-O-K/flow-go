package storage

// All includes all the storage modules
type All struct {
	Headers                 Headers
	Guarantees              Guarantees
	Seals                   Seals
	Index                   Index
	Payloads                Payloads
	Blocks                  Blocks
	QuorumCertificates      QuorumCertificates
	Setups                  EpochSetups
	EpochCommits            EpochCommits
	Statuses                EpochStatuses
	Results                 ExecutionResults
	Receipts                ExecutionReceipts
	ChunkDataPacks          ChunkDataPacks
	Commits                 Commits
	Transactions            Transactions
	TransactionResults      TransactionResults
	LightTransactionResults LightTransactionResults
	Collections             Collections
	Events                  Events
	VersionBeacons          VersionBeacons
	RegisterIndex           RegisterIndex
}
