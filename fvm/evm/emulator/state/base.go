package state

import (
	"fmt"

	"github.com/holiman/uint256"
	"github.com/onflow/atree"
	"github.com/onflow/go-ethereum/common"
	gethCommon "github.com/onflow/go-ethereum/common"
	gethTypes "github.com/onflow/go-ethereum/core/types"
	gethCrypto "github.com/onflow/go-ethereum/crypto"

	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

const (
	// AccountsStorageIDKey is the path where we store the collection ID for accounts
	AccountsStorageIDKey = "AccountsStorageIDKey"
	// CodesStorageIDKey is the path where we store the collection ID for codes
	CodesStorageIDKey = "CodesStorageIDKey"
)

var EmptyHash = gethCommon.Hash{}

// BaseView implements a types.BaseView
// it acts as the base layer of state queries for the stateDB
// it stores accounts, codes and storage slots.
//
// under the hood it uses a set of collections,
// one for account's meta data, one for codes
// and one for each of account storage space.
type BaseView struct {
	rootAddress        flow.Address
	ledger             atree.Ledger
	collectionProvider *CollectionProvider

	// collections
	accounts *Collection
	codes    *Collection
	slots    map[gethCommon.Address]*Collection

	// cached values
	cachedAccounts map[gethCommon.Address]*Account
	cachedCodes    map[gethCommon.Address][]byte
	cachedSlots    map[types.SlotAddress]gethCommon.Hash

	// flags
	accountSetupOnCommit bool
	codeSetupOnCommit    bool
}

var _ types.BaseView = &BaseView{}

// NewBaseView constructs a new base view
func NewBaseView(ledger atree.Ledger, rootAddress flow.Address) (*BaseView, error) {
	cp, err := NewCollectionProvider(atree.Address(rootAddress), ledger)
	if err != nil {
		return nil, err
	}

	view := &BaseView{
		ledger:             ledger,
		rootAddress:        rootAddress,
		collectionProvider: cp,

		slots: make(map[gethCommon.Address]*Collection),

		cachedAccounts: make(map[gethCommon.Address]*Account),
		cachedCodes:    make(map[gethCommon.Address][]byte),
		cachedSlots:    make(map[types.SlotAddress]gethCommon.Hash),
	}

	// fetch the account collection, if not exist, create one
	view.accounts, view.accountSetupOnCommit, err = view.fetchOrCreateCollection(AccountsStorageIDKey)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch or create account collection with key %v: %w", AccountsStorageIDKey, err)
	}

	// fetch the code collection, if not exist, create one
	view.codes, view.codeSetupOnCommit, err = view.fetchOrCreateCollection(CodesStorageIDKey)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch or create code collection with key %v: %w", CodesStorageIDKey, err)
	}

	return view, nil
}

// Exist returns true if the address exist in the state
func (v *BaseView) Exist(addr gethCommon.Address) (bool, error) {
	acc, err := v.getAccount(addr)
	return acc != nil, err
}

// IsCreated returns true if the address has been created in the context of this transaction
func (v *BaseView) IsCreated(gethCommon.Address) bool {
	return false
}

// IsNewContract returns true if the address is a new contract
func (v *BaseView) IsNewContract(gethCommon.Address) bool {
	return false
}

// HasSelfDestructed returns true if an address is flagged for destruction at the end of transaction
func (v *BaseView) HasSelfDestructed(gethCommon.Address) (bool, *uint256.Int) {
	return false, new(uint256.Int)
}

// GetBalance returns the balance of an address
//
// for non-existent accounts it returns a balance of zero
func (v *BaseView) GetBalance(addr gethCommon.Address) (*uint256.Int, error) {
	acc, err := v.getAccount(addr)
	bal := uint256.NewInt(0)
	if acc != nil {
		bal = acc.Balance
	}
	return bal, err
}

// GetNonce returns the nonce of an address
//
// for non-existent accounts it returns zero
func (v *BaseView) GetNonce(addr gethCommon.Address) (uint64, error) {
	acc, err := v.getAccount(addr)
	nonce := uint64(0)
	if acc != nil {
		nonce = acc.Nonce
	}
	return nonce, err
}

// GetCode returns the code of an address
//
// for non-existent accounts or accounts without a code (e.g. EOAs) it returns nil
func (v *BaseView) GetCode(addr gethCommon.Address) ([]byte, error) {
	return v.getCode(addr)
}

// GetCodeHash returns the code hash of an address
//
// for non-existent accounts it returns gethCommon.Hash{}
// and for accounts without a code (e.g. EOAs) it returns default empty
// hash value (gethTypes.EmptyCodeHash)
func (v *BaseView) GetCodeHash(addr gethCommon.Address) (gethCommon.Hash, error) {
	acc, err := v.getAccount(addr)
	codeHash := gethCommon.Hash{}
	if acc != nil {
		codeHash = acc.CodeHash
	}
	return codeHash, err
}

// GetCodeSize returns the code size of an address
//
// for non-existent accounts or accounts without a code (e.g. EOAs) it returns zero
func (v *BaseView) GetCodeSize(addr gethCommon.Address) (int, error) {
	code, err := v.GetCode(addr)
	return len(code), err
}

// GetState returns values for a slot in the main storage
//
// for non-existent slots it returns the default empty hash value (gethTypes.EmptyCodeHash)
func (v *BaseView) GetState(sk types.SlotAddress) (gethCommon.Hash, error) {
	return v.getSlot(sk)
}

// GetStorageRoot returns some sort of storage root for the given address
// WARNING! the root that is returned is not a commitment to the state
// Mostly is returned to satisfy the requirements of the EVM,
// where the returned value is compared against empty hash and empty root hash
// to determine smart contracts that already has data.
//
// Since BaseView doesn't construct a Merkel tree
// for each account hash of root slab as some sort of root hash.
// if account doesn't exist we return empty hash
// if account exist but not a smart contract we return EmptyRootHash
// if is a contract we return the hash of the root slab content (some sort of commitment).
func (v *BaseView) GetStorageRoot(addr common.Address) (common.Hash, error) {
	account, err := v.getAccount(addr)
	if err != nil {
		return gethCommon.Hash{}, err
	}
	// account does not exist
	if account == nil {
		return gethCommon.Hash{}, nil
	}

	// account is EOA
	if len(account.CollectionID) == 0 {
		return gethTypes.EmptyRootHash, nil
	}

	// otherwise is smart contract account
	// return the hash of collection ID
	// This is not a proper root as it doesn't have
	// any commitment to the content.
	return gethCrypto.Keccak256Hash(account.CollectionID), nil
}

// UpdateSlot updates the value for a slot
func (v *BaseView) UpdateSlot(sk types.SlotAddress, value gethCommon.Hash) error {
	return v.storeSlot(sk, value)
}

// GetRefund returns the total amount of (gas) refund
//
// this method returns the value of zero
func (v *BaseView) GetRefund() uint64 {
	return 0
}

// GetTransientState returns values for an slot transient storage
//
// transient storage is not a functionality for the base view so it always
// returns the default value for non-existent slots
func (v *BaseView) GetTransientState(types.SlotAddress) gethCommon.Hash {
	return gethCommon.Hash{}
}

// AddressInAccessList checks if an address is in the access list
//
// access list control is not a functionality of the base view
// it always returns false
func (v *BaseView) AddressInAccessList(gethCommon.Address) bool {
	return false
}

// SlotInAccessList checks if a slot is in the access list
//
// access list control is not a functionality of the base view
// it always returns false
func (v *BaseView) SlotInAccessList(types.SlotAddress) (addressOk bool, slotOk bool) {
	return false, false
}

// CreateAccount creates a new account
func (v *BaseView) CreateAccount(
	addr gethCommon.Address,
	balance *uint256.Int,
	nonce uint64,
	code []byte,
	codeHash gethCommon.Hash,
) error {
	var colID []byte
	// if is an smart contract account
	if len(code) > 0 {
		err := v.updateAccountCode(addr, code, codeHash)
		if err != nil {
			return err
		}
	}

	// create a new account and store it
	acc := NewAccount(addr, balance, nonce, codeHash, colID)

	// no need to update the cache , storeAccount would update the cache
	return v.storeAccount(acc)
}

// UpdateAccount updates an account's meta data
func (v *BaseView) UpdateAccount(
	addr gethCommon.Address,
	balance *uint256.Int,
	nonce uint64,
	code []byte,
	codeHash gethCommon.Hash,
) error {
	acc, err := v.getAccount(addr)
	if err != nil {
		return err
	}
	// if update is called on a non existing account
	// we gracefully call the create account
	// TODO: but we might need to revisit this action in the future
	if acc == nil {
		return v.CreateAccount(addr, balance, nonce, code, codeHash)
	}

	// update account code
	err = v.updateAccountCode(addr, code, codeHash)
	if err != nil {
		return err
	}
	// TODO: maybe purge the state in the future as well
	// currently the behavior of stateDB doesn't purge the data
	// We don't need to check if the code is empty and we purge the state
	// this is not possible right now.

	newAcc := NewAccount(addr, balance, nonce, codeHash, acc.CollectionID)
	// no need to update the cache , storeAccount would update the cache
	return v.storeAccount(newAcc)
}

// DeleteAccount deletes an account's meta data, code, and
// storage slots associated with that address
func (v *BaseView) DeleteAccount(addr gethCommon.Address) error {
	// 1. check account exists
	acc, err := v.getAccount(addr)
	if err != nil {
		return err
	}
	if acc == nil { // if account doesn't exist return
		return nil
	}

	// 2. remove the code
	if acc.HasCode() {
		err = v.updateAccountCode(addr, nil, gethTypes.EmptyCodeHash)
		if err != nil {
			return err
		}
	}

	// 3. update the cache
	delete(v.cachedAccounts, addr)

	// 4. collections
	err = v.accounts.Remove(addr.Bytes())
	if err != nil {
		return err
	}

	// 5. remove storage slots
	if len(acc.CollectionID) > 0 {
		col, found := v.slots[addr]
		if !found {
			col, err = v.collectionProvider.CollectionByID(acc.CollectionID)
			if err != nil {
				return err
			}
		}
		// delete all slots related to this account (eip-6780)
		keys, err := col.Destroy()
		if err != nil {
			return err
		}

		delete(v.slots, addr)

		for _, key := range keys {
			delete(v.cachedSlots, types.SlotAddress{
				Address: addr,
				Key:     gethCommon.BytesToHash(key),
			})
		}
	}
	return nil
}

// PurgeAllSlotsOfAnAccount purges all the slots related to an account
func (v *BaseView) PurgeAllSlotsOfAnAccount(addr gethCommon.Address) error {
	acc, err := v.getAccount(addr)
	if err != nil {
		return err
	}
	if acc == nil { // if account doesn't exist return
		return nil
	}
	col, err := v.collectionProvider.CollectionByID(acc.CollectionID)
	if err != nil {
		return err
	}
	// delete all slots related to this account (eip-6780)
	keys, err := col.Destroy()
	if err != nil {
		return err
	}
	delete(v.slots, addr)
	for _, key := range keys {
		delete(v.cachedSlots, types.SlotAddress{
			Address: addr,
			Key:     gethCommon.BytesToHash(key),
		})
	}
	return nil
}

// Commit commits the changes to the underlying storage layers
func (v *BaseView) Commit() error {
	// commit collection changes
	err := v.collectionProvider.Commit()
	if err != nil {
		return err
	}

	// if this is the first time we are setting up an
	// account collection, store its collection id.
	if v.accountSetupOnCommit {
		err = v.ledger.SetValue(v.rootAddress[:], []byte(AccountsStorageIDKey), v.accounts.CollectionID())
		if err != nil {
			return err
		}
		v.accountSetupOnCommit = false

	}

	// if this is the first time we are setting up an
	// code collection, store its collection id.
	if v.codeSetupOnCommit {
		err = v.ledger.SetValue(v.rootAddress[:], []byte(CodesStorageIDKey), v.codes.CollectionID())
		if err != nil {
			return err
		}
		v.codeSetupOnCommit = false
	}
	return nil
}

// NumberOfContracts returns the number of unique contracts
func (v *BaseView) NumberOfContracts() uint64 {
	return v.codes.Size()
}

// NumberOfContracts returns the number of accounts
func (v *BaseView) NumberOfAccounts() uint64 {
	return v.accounts.Size()
}

// AccountIterator returns an account iterator
//
// Warning! this is an expensive operation and should only be used
// for testing and exporting state operations, while no changes
// are applied to accounts. Note that the iteration order is not guaranteed.
func (v *BaseView) AccountIterator() (*AccountIterator, error) {
	itr, err := v.accounts.ReadOnlyIterator()
	if err != nil {
		return nil, err
	}
	return &AccountIterator{colIterator: itr}, nil
}

// CodeIterator returns a code iterator
//
// Warning! this is an expensive operation and should only be used
// for testing and exporting state operations, while no changes
// are applied to codes. Note that the iteration order is not guaranteed.
func (v *BaseView) CodeIterator() (*CodeIterator, error) {
	itr, err := v.codes.ReadOnlyIterator()
	if err != nil {
		return nil, err
	}
	return &CodeIterator{colIterator: itr}, nil
}

// AccountStorageIterator returns an account storage iterator
// for the given address
//
// Warning! this is an expensive operation and should only be used
// for testing and exporting state operations, while no changes
// are applied to accounts. Note that the iteration order is not guaranteed.
func (v *BaseView) AccountStorageIterator(
	addr gethCommon.Address,
) (*AccountStorageIterator, error) {
	acc, err := v.getAccount(addr)
	if err != nil {
		return nil, err
	}
	if acc == nil || !acc.HasStoredValues() {
		return nil, fmt.Errorf("account %s has no stored value", addr.String())
	}
	col, found := v.slots[addr]
	if !found {
		col, err = v.collectionProvider.CollectionByID(acc.CollectionID)
		if err != nil {
			return nil, fmt.Errorf("failed to load storage collection for account %s: %w", addr.String(), err)
		}
	}
	itr, err := col.ReadOnlyIterator()
	if err != nil {
		return nil, err
	}
	return &AccountStorageIterator{
		address:     addr,
		colIterator: itr,
	}, nil
}

func (v *BaseView) fetchOrCreateCollection(path string) (collection *Collection, created bool, error error) {
	collectionID, err := v.ledger.GetValue(v.rootAddress[:], []byte(path))
	if err != nil {
		return nil, false, err
	}
	if len(collectionID) == 0 {
		collection, err = v.collectionProvider.NewCollection()
		if err != nil {
			return collection, true, fmt.Errorf("fail to create collection with key %v: %w", path, err)
		}
		return collection, true, nil
	}
	collection, err = v.collectionProvider.CollectionByID(collectionID)
	return collection, false, err
}

func (v *BaseView) getAccount(addr gethCommon.Address) (*Account, error) {
	// check cached accounts first
	acc, found := v.cachedAccounts[addr]
	if found {
		return acc, nil
	}

	// then collect it from the account collection
	data, err := v.accounts.Get(addr.Bytes())
	if err != nil {
		return nil, err
	}
	// decode it
	acc, err = DecodeAccount(data)
	if err != nil {
		return nil, err
	}
	// cache it
	if acc != nil {
		v.cachedAccounts[addr] = acc
	}
	return acc, nil
}

func (v *BaseView) storeAccount(acc *Account) error {
	data, err := acc.Encode()
	if err != nil {
		return err
	}
	// update the cache
	v.cachedAccounts[acc.Address] = acc
	return v.accounts.Set(acc.Address.Bytes(), data)
}

func (v *BaseView) getCode(addr gethCommon.Address) ([]byte, error) {
	// check the cache first
	code, found := v.cachedCodes[addr]
	if found {
		return code, nil
	}

	// get account
	acc, err := v.getAccount(addr)
	if err != nil {
		return nil, err
	}

	if acc == nil || !acc.HasCode() {
		return nil, nil
	}

	// collect the container from the code collection by codeHash
	encoded, err := v.codes.Get(acc.CodeHash.Bytes())
	if err != nil {
		return nil, err
	}
	if len(encoded) == 0 {
		return nil, nil
	}

	codeCont, err := CodeContainerFromEncoded(encoded)
	if err != nil {
		return nil, err
	}
	code = codeCont.Code()
	if len(code) > 0 {
		v.cachedCodes[addr] = code
	}
	return code, nil
}

func (v *BaseView) updateAccountCode(addr gethCommon.Address, code []byte, codeHash gethCommon.Hash) error {
	// get account
	acc, err := v.getAccount(addr)
	if err != nil {
		return err
	}
	// if is a new account
	if acc == nil {
		if len(code) == 0 {
			return nil
		}
		v.cachedCodes[addr] = code
		return v.addCode(code, codeHash)
	}

	// skip if is the same code
	if acc.CodeHash == codeHash {
		return nil
	}

	// clean old code first if exist
	if acc.HasCode() {
		delete(v.cachedCodes, addr)
		err = v.removeCode(acc.CodeHash)
		if err != nil {
			return err
		}
	}

	// add new code
	if len(code) == 0 {
		return nil
	}
	v.cachedCodes[addr] = code
	return v.addCode(code, codeHash)
}

func (v *BaseView) removeCode(codeHash gethCommon.Hash) error {
	encoded, err := v.codes.Get(codeHash.Bytes())
	if err != nil {
		return err
	}
	if len(encoded) == 0 {
		return nil
	}

	cc, err := CodeContainerFromEncoded(encoded)
	if err != nil {
		return err
	}
	if cc.DecRefCount() {
		return v.codes.Remove(codeHash.Bytes())
	}
	return v.codes.Set(codeHash.Bytes(), cc.Encode())
}

func (v *BaseView) addCode(code []byte, codeHash gethCommon.Hash) error {
	encoded, err := v.codes.Get(codeHash.Bytes())
	if err != nil {
		return err
	}
	// if is the first time the code is getting deployed
	if len(encoded) == 0 {
		return v.codes.Set(codeHash.Bytes(), NewCodeContainer(code).Encode())
	}

	// otherwise update the cc
	cc, err := CodeContainerFromEncoded(encoded)
	if err != nil {
		return err
	}
	cc.IncRefCount()
	return v.codes.Set(codeHash.Bytes(), cc.Encode())
}

func (v *BaseView) getSlot(sk types.SlotAddress) (gethCommon.Hash, error) {
	value, found := v.cachedSlots[sk]
	if found {
		return value, nil
	}

	acc, err := v.getAccount(sk.Address)
	if err != nil {
		return gethCommon.Hash{}, err
	}
	if acc == nil || len(acc.CollectionID) == 0 {
		return gethCommon.Hash{}, nil
	}

	col, err := v.getSlotCollection(acc)
	if err != nil {
		return gethCommon.Hash{}, err
	}

	val, err := col.Get(sk.Key.Bytes())
	if err != nil {
		return gethCommon.Hash{}, err
	}
	value = gethCommon.BytesToHash(val)
	v.cachedSlots[sk] = value
	return value, nil
}

func (v *BaseView) storeSlot(sk types.SlotAddress, data gethCommon.Hash) error {
	acc, err := v.getAccount(sk.Address)
	if err != nil {
		return err
	}
	if acc == nil {
		return fmt.Errorf("slot belongs to a non-existing account")
	}
	if !acc.HasCode() {
		return fmt.Errorf("slot belongs to a non-smart contract account")
	}
	col, err := v.getSlotCollection(acc)
	if err != nil {
		return err
	}

	if data == EmptyHash {
		delete(v.cachedSlots, sk)
		return col.Remove(sk.Key.Bytes())
	}
	v.cachedSlots[sk] = data
	return col.Set(sk.Key.Bytes(), data.Bytes())
}

func (v *BaseView) getSlotCollection(acc *Account) (*Collection, error) {
	var err error

	if len(acc.CollectionID) == 0 {
		// create a new collection for slots
		col, err := v.collectionProvider.NewCollection()
		if err != nil {
			return nil, err
		}
		// cache collection
		v.slots[acc.Address] = col
		// update account's collection ID
		acc.CollectionID = col.CollectionID()
		err = v.storeAccount(acc)
		if err != nil {
			return nil, err
		}
		return col, nil
	}

	col, found := v.slots[acc.Address]
	if !found {
		col, err = v.collectionProvider.CollectionByID(acc.CollectionID)
		if err != nil {
			return nil, err
		}
		v.slots[acc.Address] = col
	}
	return col, nil
}

// AccountIterator iterates over accounts
type AccountIterator struct {
	colIterator *CollectionIterator
}

// Next returns the next account
// if no more accounts next would return nil (no error)
func (ai *AccountIterator) Next() (*Account, error) {
	_, value, err := ai.colIterator.Next()
	if err != nil {
		return nil, fmt.Errorf("account iteration failed: %w", err)
	}
	return DecodeAccount(value)
}

// CodeIterator iterates over codes stored in EVM
// code storage only stores unique codes
type CodeIterator struct {
	colIterator *CollectionIterator
}

// Next returns the next code
// if no more codes, it return nil (no error)
func (ci *CodeIterator) Next() (
	*CodeInContext,
	error,
) {
	ch, encodedCC, err := ci.colIterator.Next()
	if err != nil {
		return nil, fmt.Errorf("code iteration failed: %w", err)
	}
	// no more keys
	if ch == nil {
		return nil, nil
	}
	if len(encodedCC) == 0 {
		return nil,
			fmt.Errorf("encoded code container is empty (code hash: %x)", ch)
	}

	codeCont, err := CodeContainerFromEncoded(encodedCC)
	if err != nil {
		return nil, fmt.Errorf("code container decoding failed (code hash: %x)", ch)

	}
	return &CodeInContext{
		Hash:      gethCommon.BytesToHash(ch),
		Code:      codeCont.Code(),
		RefCounts: codeCont.RefCount(),
	}, nil
}

// AccountStorageIterator iterates over slots of an account
type AccountStorageIterator struct {
	address     gethCommon.Address
	colIterator *CollectionIterator
}

// Next returns the next slot in the storage
// if no more keys, it returns nil (no error)
func (asi *AccountStorageIterator) Next() (
	*types.SlotEntry,
	error,
) {
	k, v, err := asi.colIterator.Next()
	if err != nil {
		return nil, fmt.Errorf("account storage iteration failed: %w", err)
	}
	// no more keys
	if k == nil {
		return nil, nil
	}
	return &types.SlotEntry{
		Address: asi.address,
		Key:     gethCommon.BytesToHash(k),
		Value:   gethCommon.BytesToHash(v),
	}, nil

}
