package fvm_test

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
	"testing"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/engine/execution/testutil"
	exeUtils "github.com/onflow/flow-go/engine/execution/utils"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/blueprints"
	crypto2 "github.com/onflow/flow-go/fvm/crypto"
	errors "github.com/onflow/flow-go/fvm/errors"
	fvmmock "github.com/onflow/flow-go/fvm/mock"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/utils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

type vmTest struct {
	bootstrapOptions []fvm.BootstrapProcedureOption
	contextOptions   []fvm.Option
}

func newVMTest() vmTest {
	return vmTest{}
}

func (vmt vmTest) withBootstrapProcedureOptions(opts ...fvm.BootstrapProcedureOption) vmTest {
	vmt.bootstrapOptions = append(vmt.bootstrapOptions, opts...)
	return vmt
}

func (vmt vmTest) withContextOptions(opts ...fvm.Option) vmTest {
	vmt.contextOptions = append(vmt.contextOptions, opts...)
	return vmt
}

func (vmt vmTest) run(
	f func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs),
) func(t *testing.T) {
	return func(t *testing.T) {
		rt := fvm.NewInterpreterRuntime()

		chain := flow.Testnet.Chain()

		vm := fvm.NewVirtualMachine(rt)

		baseOpts := []fvm.Option{
			fvm.WithChain(chain),
		}

		opts := append(baseOpts, vmt.contextOptions...)

		ctx := fvm.NewContext(zerolog.Nop(), opts...)

		view := utils.NewSimpleView()

		baseBootstrapOpts := []fvm.BootstrapProcedureOption{
			fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
		}

		programs := programs.NewEmptyPrograms()

		bootstrapOpts := append(baseBootstrapOpts, vmt.bootstrapOptions...)

		err := vm.Run(ctx, fvm.Bootstrap(unittest.ServiceAccountPublicKey, bootstrapOpts...), view, programs)
		require.NoError(t, err)

		f(t, vm, chain, ctx, view, programs)
	}
}

func transferTokensTx(chain flow.Chain) *flow.TransactionBody {
	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(`
							// This transaction is a template for a transaction that
							// could be used by anyone to send tokens to another account
							// that has been set up to receive tokens.
							//
							// The withdraw amount and the account from getAccount
							// would be the parameters to the transaction
							
							import FungibleToken from 0x%s
							import FlowToken from 0x%s
							
							transaction(amount: UFix64, to: Address) {
							
								// The Vault resource that holds the tokens that are being transferred
								let sentVault: @FungibleToken.Vault
							
								prepare(signer: AuthAccount) {
							
									// Get a reference to the signer's stored vault
									let vaultRef = signer.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)
										?? panic("Could not borrow reference to the owner's Vault!")
							
									// Withdraw tokens from the signer's stored vault
									self.sentVault <- vaultRef.withdraw(amount: amount)
								}
							
								execute {
							
									// Get the recipient's public account object
									let recipient = getAccount(to)
							
									// Get a reference to the recipient's Receiver
									let receiverRef = recipient.getCapability(/public/flowTokenReceiver)
										.borrow<&{FungibleToken.Receiver}>()
										?? panic("Could not borrow receiver reference to the recipient's Vault")
							
									// Deposit the withdrawn tokens in the recipient's receiver
									receiverRef.deposit(from: <-self.sentVault)
								}
							}`, fvm.FungibleTokenAddress(chain), fvm.FlowTokenAddress(chain))),
		)
}

func filterAccountCreatedEvents(events []flow.Event) []flow.Event {
	var accountCreatedEvents []flow.Event
	for _, event := range events {
		if event.Type != flow.EventAccountCreated {
			continue
		}
		accountCreatedEvents = append(accountCreatedEvents, event)
		break
	}
	return accountCreatedEvents
}

func TestPrograms(t *testing.T) {

	t.Run(
		"transaction execution programs are committed",
		newVMTest().run(
			func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {

				txCtx := fvm.NewContextFromParent(ctx)

				for i := 0; i < 10; i++ {

					script := []byte(fmt.Sprintf(`
							import FungibleToken from %s

							transaction {}
						`,
						fvm.FungibleTokenAddress(chain).HexWithPrefix(),
					))

					serviceAddress := chain.ServiceAddress()

					txBody := flow.NewTransactionBody().
						SetScript(script).
						SetProposalKey(serviceAddress, 0, uint64(i)).
						SetPayer(serviceAddress)

					err := testutil.SignEnvelope(
						txBody,
						serviceAddress,
						unittest.ServiceAccountPrivateKey,
					)
					require.NoError(t, err)

					tx := fvm.Transaction(txBody, uint32(i))

					err = vm.Run(txCtx, tx, view, programs)
					require.NoError(t, err)

					require.NoError(t, tx.Err)
				}
			},
		),
	)

	t.Run("script execution programs are not committed",
		newVMTest().run(
			func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {

				scriptCtx := fvm.NewContextFromParent(ctx)

				script := fvm.Script([]byte(fmt.Sprintf(`

						import FungibleToken from %s

						pub fun main() {}
					`,
					fvm.FungibleTokenAddress(chain).HexWithPrefix(),
				)))

				err := vm.Run(scriptCtx, script, view, programs)
				require.NoError(t, err)
				require.NoError(t, script.Err)
			},
		),
	)
}

func TestBlockContext_ExecuteTransaction(t *testing.T) {

	t.Parallel()

	rt := fvm.NewInterpreterRuntime()

	chain := flow.Testnet.Chain()

	vm := fvm.NewVirtualMachine(rt)

	ctx := fvm.NewContext(
		zerolog.Nop(),
		fvm.WithChain(chain),
		fvm.WithCadenceLogging(true),
	)

	t.Run("Success", func(t *testing.T) {
		txBody := flow.NewTransactionBody().
			SetScript([]byte(`
	            transaction {
	              prepare(signer: AuthAccount) {}
	            }
	        `)).
			AddAuthorizer(unittest.AddressFixture())

		err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
		require.NoError(t, err)

		view := testutil.RootBootstrappedLedger(vm, ctx)
		tx := fvm.Transaction(txBody, 0)

		err = vm.Run(ctx, tx, view, programs.NewEmptyPrograms())
		require.NoError(t, err)

		assert.Nil(t, tx.Err)
	})

	t.Run("Failure", func(t *testing.T) {
		txBody := flow.NewTransactionBody().
			SetScript([]byte(`
                transaction {
                  var x: Int

                  prepare(signer: AuthAccount) {
                    self.x = 0
                  }

                  execute {
                    self.x = 1
                  }

                  post {
                    self.x == 2
                  }
                }
            `))

		err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
		require.NoError(t, err)

		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		tx := fvm.Transaction(txBody, 0)

		err = vm.Run(ctx, tx, ledger, programs.NewEmptyPrograms())
		require.NoError(t, err)

		assert.Error(t, tx.Err)
	})

	t.Run("Logs", func(t *testing.T) {
		txBody := flow.NewTransactionBody().
			SetScript([]byte(`
                transaction {
                  execute {
                    log("foo")
                    log("bar")
                  }
                }
            `))

		err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
		require.NoError(t, err)

		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		tx := fvm.Transaction(txBody, 0)

		err = vm.Run(ctx, tx, ledger, programs.NewEmptyPrograms())
		require.NoError(t, err)

		require.Len(t, tx.Logs, 2)
		assert.Equal(t, "\"foo\"", tx.Logs[0])
		assert.Equal(t, "\"bar\"", tx.Logs[1])
	})

	t.Run("Events", func(t *testing.T) {
		txBody := flow.NewTransactionBody().
			SetScript([]byte(`
                transaction {
                  prepare(signer: AuthAccount) {
                    AuthAccount(payer: signer)
                  }
                }
            `)).
			AddAuthorizer(chain.ServiceAddress())

		err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
		require.NoError(t, err)

		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		tx := fvm.Transaction(txBody, 0)

		err = vm.Run(ctx, tx, ledger, programs.NewEmptyPrograms())
		require.NoError(t, err)

		assert.NoError(t, tx.Err)

		accountCreatedEvents := filterAccountCreatedEvents(tx.Events)

		require.Len(t, accountCreatedEvents, 1)
	})
}

func TestBlockContext_DeployContract(t *testing.T) {

	t.Parallel()

	rt := fvm.NewInterpreterRuntime()

	chain := flow.Mainnet.Chain()

	vm := fvm.NewVirtualMachine(rt)

	ctx := fvm.NewContext(
		zerolog.Nop(),
		fvm.WithChain(chain),
		fvm.WithCadenceLogging(true),
	)

	t.Run("account update with set code succeeds as service account", func(t *testing.T) {
		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		// Create an account private key.
		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
		accounts, err := testutil.CreateAccounts(vm, ledger, programs.NewEmptyPrograms(), privateKeys, chain)
		require.NoError(t, err)

		txBody := testutil.DeployCounterContractTransaction(accounts[0], chain)

		txBody.SetProposalKey(chain.ServiceAddress(), 0, 0)
		txBody.SetPayer(chain.ServiceAddress())

		err = testutil.SignPayload(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		err = testutil.SignEnvelope(txBody, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
		require.NoError(t, err)

		tx := fvm.Transaction(txBody, 0)

		err = vm.Run(ctx, tx, ledger, programs.NewEmptyPrograms())
		require.NoError(t, err)

		assert.NoError(t, tx.Err)
	})

	t.Run("account with deployed contract has `contracts.names` filled", func(t *testing.T) {
		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		// Create an account private key.
		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
		accounts, err := testutil.CreateAccounts(vm, ledger, programs.NewEmptyPrograms(), privateKeys, chain)
		require.NoError(t, err)

		txBody := testutil.DeployCounterContractTransaction(accounts[0], chain)

		txBody.SetProposalKey(chain.ServiceAddress(), 0, 0)
		txBody.SetPayer(chain.ServiceAddress())

		err = testutil.SignPayload(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		err = testutil.SignEnvelope(txBody, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
		require.NoError(t, err)

		tx := fvm.Transaction(txBody, 0)

		err = vm.Run(ctx, tx, ledger, programs.NewEmptyPrograms())
		require.NoError(t, err)

		assert.NoError(t, tx.Err)

		// transaction will panic if `contracts.names` is incorrect
		txBody = flow.NewTransactionBody().
			SetScript([]byte(`
				transaction {
					prepare(signer: AuthAccount) {
						var s : String = ""
						for name in signer.contracts.names {
							s = s.concat(name).concat(",")
						}
						if s != "Container," {
							panic(s)
						}
					}
				}
			`)).
			AddAuthorizer(accounts[0])

		txBody.SetProposalKey(chain.ServiceAddress(), 0, 1)
		txBody.SetPayer(chain.ServiceAddress())

		err = testutil.SignPayload(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		err = testutil.SignEnvelope(txBody, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
		require.NoError(t, err)

		tx = fvm.Transaction(txBody, 0)

		err = vm.Run(ctx, tx, ledger, programs.NewEmptyPrograms())
		require.NoError(t, err)

		assert.NoError(t, tx.Err)
	})

	t.Run("account update with checker heavy contract", func(t *testing.T) {
		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		// Create an account private key.
		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
		accounts, err := testutil.CreateAccounts(vm, ledger, programs.NewEmptyPrograms(), privateKeys, chain)
		require.NoError(t, err)

		txBody := testutil.DeployCheckerHeavyTransaction(accounts[0], chain)

		txBody.SetProposalKey(chain.ServiceAddress(), 0, 0)
		txBody.SetPayer(chain.ServiceAddress())

		err = testutil.SignPayload(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		err = testutil.SignEnvelope(txBody, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
		require.NoError(t, err)

		tx := fvm.Transaction(txBody, 0)

		err = vm.Run(ctx, tx, ledger, programs.NewEmptyPrograms())
		require.NoError(t, err)

		assert.NoError(t, tx.Err)
	})

	t.Run("account update with set code fails if not signed by service account", func(t *testing.T) {
		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		// Create an account private key.
		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
		accounts, err := testutil.CreateAccounts(vm, ledger, programs.NewEmptyPrograms(), privateKeys, chain)
		require.NoError(t, err)

		txBody := testutil.DeployUnauthorizedCounterContractTransaction(accounts[0])

		err = testutil.SignTransaction(txBody, accounts[0], privateKeys[0], 0)
		require.NoError(t, err)

		tx := fvm.Transaction(txBody, 0)

		err = vm.Run(ctx, tx, ledger, programs.NewEmptyPrograms())
		require.NoError(t, err)

		assert.Error(t, tx.Err)

		assert.Contains(t, tx.Err.Error(), "setting contracts requires authorization from specific accounts")
		assert.Equal(t, (&errors.CadenceRuntimeError{}).Code(), tx.Err.Code())
	})

	t.Run("account update with set code fails if not signed by service account", func(t *testing.T) {
		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		// Create an account private key.
		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
		accounts, err := testutil.CreateAccounts(vm, ledger, programs.NewEmptyPrograms(), privateKeys, chain)
		require.NoError(t, err)

		txBody := testutil.DeployUnauthorizedCounterContractTransaction(accounts[0])

		err = testutil.SignTransaction(txBody, accounts[0], privateKeys[0], 0)
		require.NoError(t, err)

		tx := fvm.Transaction(txBody, 0)

		err = vm.Run(ctx, tx, ledger, programs.NewEmptyPrograms())
		require.NoError(t, err)

		assert.Error(t, tx.Err)

		assert.Contains(t, tx.Err.Error(), "setting contracts requires authorization from specific accounts")
		assert.Equal(t, (&errors.CadenceRuntimeError{}).Code(), tx.Err.Code())
	})

	t.Run("account update with set code succeeds when account is added as authorized account", func(t *testing.T) {
		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		// Create an account private key.
		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
		accounts, err := testutil.CreateAccounts(vm, ledger, programs.NewEmptyPrograms(), privateKeys, chain)
		require.NoError(t, err)

		// setup a new authorizer account
		authTxBody, err := blueprints.SetContractDeploymentAuthorizersTransaction(chain.ServiceAddress(), []flow.Address{chain.ServiceAddress(), accounts[0]})
		require.NoError(t, err)

		authTxBody.SetProposalKey(chain.ServiceAddress(), 0, 0)
		authTxBody.SetPayer(chain.ServiceAddress())
		err = testutil.SignEnvelope(authTxBody, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
		require.NoError(t, err)
		authTx := fvm.Transaction(authTxBody, 0)

		err = vm.Run(ctx, authTx, ledger, programs.NewEmptyPrograms())
		require.NoError(t, err)
		assert.NoError(t, authTx.Err)

		// test deploying a new contract (not authorized by service account)
		txBody := testutil.DeployUnauthorizedCounterContractTransaction(accounts[0])
		txBody.SetProposalKey(accounts[0], 0, 0)
		txBody.SetPayer(accounts[0])

		err = testutil.SignEnvelope(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		tx := fvm.Transaction(txBody, 0)
		err = vm.Run(ctx, tx, ledger, programs.NewEmptyPrograms())
		require.NoError(t, err)
		assert.NoError(t, tx.Err)
	})

}

func TestBlockContext_ExecuteTransaction_WithArguments(t *testing.T) {

	t.Parallel()

	rt := fvm.NewInterpreterRuntime()

	chain := flow.Mainnet.Chain()

	vm := fvm.NewVirtualMachine(rt)

	ctx := fvm.NewContext(
		zerolog.Nop(),
		fvm.WithChain(chain),
		fvm.WithCadenceLogging(true),
	)

	arg1, _ := jsoncdc.Encode(cadence.NewInt(42))
	fooString, _ := cadence.NewString("foo")
	arg2, _ := jsoncdc.Encode(fooString)

	var tests = []struct {
		label       string
		script      string
		args        [][]byte
		authorizers []flow.Address
		check       func(t *testing.T, tx *fvm.TransactionProcedure)
	}{
		{
			label:  "No parameters",
			script: `transaction { execute { log("Hello, World!") } }`,
			args:   [][]byte{arg1},
			check: func(t *testing.T, tx *fvm.TransactionProcedure) {
				assert.Error(t, tx.Err)
			},
		},
		{
			label:  "Single parameter",
			script: `transaction(x: Int) { execute { log(x) } }`,
			args:   [][]byte{arg1},
			check: func(t *testing.T, tx *fvm.TransactionProcedure) {
				require.NoError(t, tx.Err)
				require.Len(t, tx.Logs, 1)
				assert.Equal(t, "42", tx.Logs[0])
			},
		},
		{
			label:  "Multiple parameters",
			script: `transaction(x: Int, y: String) { execute { log(x); log(y) } }`,
			args:   [][]byte{arg1, arg2},
			check: func(t *testing.T, tx *fvm.TransactionProcedure) {
				require.NoError(t, tx.Err)
				require.Len(t, tx.Logs, 2)
				assert.Equal(t, "42", tx.Logs[0])
				assert.Equal(t, `"foo"`, tx.Logs[1])
			},
		},
		{
			label: "Parameters and authorizer",
			script: `
                transaction(x: Int, y: String) {
                    prepare(acct: AuthAccount) { log(acct.address) }
                    execute { log(x); log(y) }
                }`,
			args:        [][]byte{arg1, arg2},
			authorizers: []flow.Address{chain.ServiceAddress()},
			check: func(t *testing.T, tx *fvm.TransactionProcedure) {
				require.NoError(t, tx.Err)
				assert.ElementsMatch(t, []string{"0x" + chain.ServiceAddress().Hex(), "42", `"foo"`}, tx.Logs)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			txBody := flow.NewTransactionBody().
				SetScript([]byte(tt.script)).
				SetArguments(tt.args)

			for _, authorizer := range tt.authorizers {
				txBody.AddAuthorizer(authorizer)
			}

			ledger := testutil.RootBootstrappedLedger(vm, ctx)

			err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
			require.NoError(t, err)

			tx := fvm.Transaction(txBody, 0)

			err = vm.Run(ctx, tx, ledger, programs.NewEmptyPrograms())
			require.NoError(t, err)

			tt.check(t, tx)
		})
	}
}

func gasLimitScript(depth int) string {
	return fmt.Sprintf(`
        pub fun foo(_ i: Int) {
            if i <= 0 {
                return
            }
            log("foo")
            foo(i-1)
        }

        transaction { execute { foo(%d) } }
    `, depth)
}

func TestBlockContext_ExecuteTransaction_GasLimit(t *testing.T) {

	t.Parallel()

	rt := fvm.NewInterpreterRuntime()

	chain := flow.Mainnet.Chain()

	vm := fvm.NewVirtualMachine(rt)

	ctx := fvm.NewContext(
		zerolog.Nop(),
		fvm.WithChain(chain),
		fvm.WithCadenceLogging(true),
	)

	var tests = []struct {
		label    string
		script   string
		gasLimit uint64
		check    func(t *testing.T, tx *fvm.TransactionProcedure)
	}{
		{
			label:    "Zero",
			script:   gasLimitScript(100),
			gasLimit: 0,
			check: func(t *testing.T, tx *fvm.TransactionProcedure) {
				// gas limit of zero is ignored by runtime
				require.NoError(t, tx.Err)
			},
		},
		{
			label:    "Insufficient",
			script:   gasLimitScript(100),
			gasLimit: 5,
			check: func(t *testing.T, tx *fvm.TransactionProcedure) {
				assert.Error(t, tx.Err)
			},
		},
		{
			label:    "Sufficient",
			script:   gasLimitScript(100),
			gasLimit: 1000,
			check: func(t *testing.T, tx *fvm.TransactionProcedure) {
				require.NoError(t, tx.Err)
				require.Len(t, tx.Logs, 100)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			txBody := flow.NewTransactionBody().
				SetScript([]byte(tt.script)).
				SetGasLimit(tt.gasLimit)

			ledger := testutil.RootBootstrappedLedger(vm, ctx)

			err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
			require.NoError(t, err)

			tx := fvm.Transaction(txBody, 0)

			err = vm.Run(ctx, tx, ledger, programs.NewEmptyPrograms())
			require.NoError(t, err)

			tt.check(t, tx)
		})
	}
}

func TestBlockContext_ExecuteTransaction_StorageLimit(t *testing.T) {

	t.Parallel()

	b := make([]byte, 1000000) // 1MB
	_, err := rand.Read(b)
	require.NoError(t, err)
	longString := base64.StdEncoding.EncodeToString(b) // 1.3 times 1MB

	script := fmt.Sprintf(`
			access(all) contract Container {
				access(all) resource Counter {
					pub var longString: String
					init() {
						self.longString = "%s"
					}
				}
			}`, longString)

	bootstrapOptions := []fvm.BootstrapProcedureOption{
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
	}

	t.Run("Storing too much data fails", newVMTest().withBootstrapProcedureOptions(bootstrapOptions...).
		run(
			func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
				ctx.LimitAccountStorage = true // this test requires storage limits to be enforced

				// Create an account private key.
				privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
				require.NoError(t, err)

				// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
				accounts, err := testutil.CreateAccounts(vm, view, programs, privateKeys, chain)
				require.NoError(t, err)

				txBody := testutil.CreateContractDeploymentTransaction(
					"Container",
					script,
					accounts[0],
					chain)

				txBody.SetProposalKey(chain.ServiceAddress(), 0, 0)
				txBody.SetPayer(chain.ServiceAddress())

				err = testutil.SignPayload(txBody, accounts[0], privateKeys[0])
				require.NoError(t, err)

				err = testutil.SignEnvelope(txBody, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
				require.NoError(t, err)

				tx := fvm.Transaction(txBody, 0)

				err = vm.Run(ctx, tx, view, programs)
				require.NoError(t, err)

				assert.Equal(t, (&errors.StorageCapacityExceededError{}).Code(), tx.Err.Code())
			}))
	t.Run("Increasing storage capacity works", newVMTest().withBootstrapProcedureOptions(bootstrapOptions...).
		run(
			func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
				ctx.LimitAccountStorage = true // this test requires storage limits to be enforced

				// Create an account private key.
				privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
				require.NoError(t, err)

				// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
				accounts, err := testutil.CreateAccounts(vm, view, programs, privateKeys, chain)
				require.NoError(t, err)

				// deposit more flow to increase capacity
				txBody := flow.NewTransactionBody().
					SetScript([]byte(fmt.Sprintf(`
					import FungibleToken from %s
					import FlowToken from %s

					transaction {
						prepare(signer: AuthAccount, service: AuthAccount) {
							signer.contracts.add(name: "%s", code: "%s".decodeHex())

							let vaultRef = service.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)!
							// deposit additional flow
							let payment <- vaultRef.withdraw(amount: 10.0) as! @FlowToken.Vault

							let receiver = signer.getCapability(/public/flowTokenReceiver)!.borrow<&{FungibleToken.Receiver}>() 
								?? panic("Could not borrow receiver reference to the recipient's Vault")
							receiver.deposit(from: <-payment)
						}
					}`, fvm.FungibleTokenAddress(chain).HexWithPrefix(),
						fvm.FlowTokenAddress(chain).HexWithPrefix(),
						"Container",
						hex.EncodeToString([]byte(script))))).
					AddAuthorizer(accounts[0]).
					AddAuthorizer(chain.ServiceAddress()).
					SetProposalKey(chain.ServiceAddress(), 0, 0).
					SetPayer(chain.ServiceAddress())

				err = testutil.SignPayload(txBody, accounts[0], privateKeys[0])
				require.NoError(t, err)

				err = testutil.SignEnvelope(txBody, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
				require.NoError(t, err)

				tx := fvm.Transaction(txBody, 0)

				err = vm.Run(ctx, tx, view, programs)
				require.NoError(t, err)

				require.NoError(t, tx.Err)
			}))
}

var createAccountScript = []byte(`
    transaction {
        prepare(signer: AuthAccount) {
            let acct = AuthAccount(payer: signer)
        }
    }
`)

func TestBlockContext_ExecuteScript(t *testing.T) {

	t.Parallel()

	rt := fvm.NewInterpreterRuntime()

	chain := flow.Mainnet.Chain()

	vm := fvm.NewVirtualMachine(rt)

	ctx := fvm.NewContext(
		zerolog.Nop(),
		fvm.WithChain(chain),
		fvm.WithCadenceLogging(true),
	)

	t.Run("script success", func(t *testing.T) {
		code := []byte(`
            pub fun main(): Int {
                return 42
            }
        `)

		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		script := fvm.Script(code)

		err := vm.Run(ctx, script, ledger, programs.NewEmptyPrograms())
		assert.NoError(t, err)

		assert.NoError(t, script.Err)
	})

	t.Run("script failure", func(t *testing.T) {
		code := []byte(`
            pub fun main(): Int {
                assert(1 == 2)
                return 42
            }
        `)

		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		script := fvm.Script(code)

		err := vm.Run(ctx, script, ledger, programs.NewEmptyPrograms())
		assert.NoError(t, err)

		assert.Error(t, script.Err)
	})

	t.Run("script logs", func(t *testing.T) {
		code := []byte(`
            pub fun main(): Int {
                log("foo")
                log("bar")
                return 42
            }
        `)

		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		script := fvm.Script(code)

		err := vm.Run(ctx, script, ledger, programs.NewEmptyPrograms())
		assert.NoError(t, err)

		assert.NoError(t, script.Err)
		require.Len(t, script.Logs, 2)
		assert.Equal(t, "\"foo\"", script.Logs[0])
		assert.Equal(t, "\"bar\"", script.Logs[1])
	})
}

func TestBlockContext_GetBlockInfo(t *testing.T) {

	t.Parallel()

	rt := fvm.NewInterpreterRuntime()

	chain := flow.Mainnet.Chain()

	vm := fvm.NewVirtualMachine(rt)

	ctx := fvm.NewContext(
		zerolog.Nop(),
		fvm.WithChain(chain),
		fvm.WithCadenceLogging(true),
	)

	blocks := new(fvmmock.Blocks)

	block1 := unittest.BlockFixture()
	block2 := unittest.BlockWithParentFixture(block1.Header)
	block3 := unittest.BlockWithParentFixture(block2.Header)

	blocks.On("ByHeightFrom", block1.Header.Height, block1.Header).Return(block1.Header, nil)
	blocks.On("ByHeightFrom", block2.Header.Height, block1.Header).Return(block2.Header, nil)

	type logPanic struct{}
	blocks.On("ByHeightFrom", block3.Header.Height, block1.Header).Run(func(args mock.Arguments) { panic(logPanic{}) })

	blockCtx := fvm.NewContextFromParent(ctx, fvm.WithBlocks(blocks), fvm.WithBlockHeader(block1.Header))

	t.Run("works as transaction", func(t *testing.T) {
		txBody := flow.NewTransactionBody().
			SetScript([]byte(`
                transaction {
                    execute {
                        let block = getCurrentBlock()
                        log(block)

                        let nextBlock = getBlock(at: block.height + UInt64(1))
                        log(nextBlock)
                    }
                }
            `))

		err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
		require.NoError(t, err)

		ledger := testutil.RootBootstrappedLedger(vm, ctx)
		require.NoError(t, err)

		tx := fvm.Transaction(txBody, 0)

		err = vm.Run(blockCtx, tx, ledger, programs.NewEmptyPrograms())
		assert.NoError(t, err)

		assert.NoError(t, tx.Err)

		require.Len(t, tx.Logs, 2)
		assert.Equal(
			t,
			fmt.Sprintf(
				"Block(height: %v, view: %v, id: 0x%x, timestamp: %.8f)",
				block1.Header.Height,
				block1.Header.View,
				block1.ID(),
				float64(block1.Header.Timestamp.Unix()),
			),
			tx.Logs[0],
		)
		assert.Equal(
			t,
			fmt.Sprintf(
				"Block(height: %v, view: %v, id: 0x%x, timestamp: %.8f)",
				block2.Header.Height,
				block2.Header.View,
				block2.ID(),
				float64(block2.Header.Timestamp.Unix()),
			),
			tx.Logs[1],
		)
	})

	t.Run("works as script", func(t *testing.T) {
		code := []byte(`
            pub fun main() {
                let block = getCurrentBlock()
                log(block)

                let nextBlock = getBlock(at: block.height + UInt64(1))
                log(nextBlock)
            }
        `)

		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		script := fvm.Script(code)

		err := vm.Run(blockCtx, script, ledger, programs.NewEmptyPrograms())
		assert.NoError(t, err)

		assert.NoError(t, script.Err)

		require.Len(t, script.Logs, 2)
		assert.Equal(t,
			fmt.Sprintf(
				"Block(height: %v, view: %v, id: 0x%x, timestamp: %.8f)",
				block1.Header.Height,
				block1.Header.View,
				block1.ID(),
				float64(block1.Header.Timestamp.Unix()),
			),
			script.Logs[0],
		)
		assert.Equal(
			t,
			fmt.Sprintf(
				"Block(height: %v, view: %v, id: 0x%x, timestamp: %.8f)",
				block2.Header.Height,
				block2.Header.View,
				block2.ID(),
				float64(block2.Header.Timestamp.Unix()),
			),
			script.Logs[1],
		)
	})

	t.Run("panics if external function panics in transaction", func(t *testing.T) {
		tx := flow.NewTransactionBody().
			SetScript([]byte(`
                transaction {
                    execute {
                        let block = getCurrentBlock()
                        let nextBlock = getBlock(at: block.height + UInt64(2))
                    }
                }
            `))

		err := testutil.SignTransactionAsServiceAccount(tx, 0, chain)
		require.NoError(t, err)

		ledger := testutil.RootBootstrappedLedger(vm, ctx)
		require.NoError(t, err)

		assert.PanicsWithValue(t, interpreter.ExternalError{
			Recovered: logPanic{},
		}, func() {
			_ = vm.Run(blockCtx, fvm.Transaction(tx, 0), ledger, programs.NewEmptyPrograms())
		})
	})

	t.Run("panics if external function panics in script", func(t *testing.T) {
		script := []byte(`
            pub fun main() {
                let block = getCurrentBlock()
                let nextBlock = getBlock(at: block.height + UInt64(2))
            }
        `)

		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		assert.PanicsWithValue(t, interpreter.ExternalError{
			Recovered: logPanic{},
		}, func() {
			_ = vm.Run(blockCtx, fvm.Script(script), ledger, programs.NewEmptyPrograms())
		})
	})
}

func TestBlockContext_GetAccount(t *testing.T) {

	t.Parallel()

	const count = 100

	rt := fvm.NewInterpreterRuntime()

	chain := flow.Mainnet.Chain()

	vm := fvm.NewVirtualMachine(rt)

	ctx := fvm.NewContext(
		zerolog.Nop(),
		fvm.WithChain(chain),
		fvm.WithCadenceLogging(true),
	)

	sequenceNumber := uint64(0)

	ledger := testutil.RootBootstrappedLedger(vm, ctx)

	programs := programs.NewEmptyPrograms()

	createAccount := func() (flow.Address, crypto.PublicKey) {
		privateKey, txBody := testutil.CreateAccountCreationTransaction(t, chain)

		txBody.SetProposalKey(chain.ServiceAddress(), 0, sequenceNumber)
		txBody.SetPayer(chain.ServiceAddress())
		sequenceNumber++

		rootHasher := hash.NewSHA2_256()

		err := txBody.SignEnvelope(
			chain.ServiceAddress(),
			0,
			unittest.ServiceAccountPrivateKey.PrivateKey,
			rootHasher,
		)
		require.NoError(t, err)

		// execute the transaction
		tx := fvm.Transaction(txBody, 0)

		err = vm.Run(ctx, tx, ledger, programs)
		require.NoError(t, err)

		assert.NoError(t, tx.Err)

		accountCreatedEvents := filterAccountCreatedEvents(tx.Events)

		require.Len(t, accountCreatedEvents, 1)

		// read the address of the account created (e.g. "0x01" and convert it to flow.address)
		data, err := jsoncdc.Decode(accountCreatedEvents[0].Payload)
		require.NoError(t, err)
		address := flow.Address(data.(cadence.Event).Fields[0].(cadence.Address))

		return address, privateKey.PublicKey(fvm.AccountKeyWeightThreshold).PublicKey
	}

	addressGen := chain.NewAddressGenerator()
	// skip the addresses of 4 reserved accounts
	for i := 0; i < 4; i++ {
		_, err := addressGen.NextAddress()
		require.NoError(t, err)
	}

	// create a bunch of accounts
	accounts := make(map[flow.Address]crypto.PublicKey, count)
	for i := 0; i < count; i++ {
		address, key := createAccount()
		expectedAddress, err := addressGen.NextAddress()
		require.NoError(t, err)

		assert.Equal(t, expectedAddress, address)
		accounts[address] = key
	}

	// happy path - get each of the created account and check if it is the right one
	t.Run("get accounts", func(t *testing.T) {
		for address, expectedKey := range accounts {

			account, err := vm.GetAccount(ctx, address, ledger, programs)
			require.NoError(t, err)

			assert.Len(t, account.Keys, 1)
			actualKey := account.Keys[0].PublicKey
			assert.Equal(t, expectedKey, actualKey)
		}
	})

	// non-happy path - get an account that was never created
	t.Run("get a non-existing account", func(t *testing.T) {
		address, err := addressGen.NextAddress()
		require.NoError(t, err)

		var account *flow.Account
		account, err = vm.GetAccount(ctx, address, ledger, programs)
		assert.True(t, errors.IsAccountNotFoundError(err))
		assert.Nil(t, account)
	})
}

func TestBlockContext_UnsafeRandom(t *testing.T) {

	t.Parallel()

	rt := fvm.NewInterpreterRuntime()

	chain := flow.Mainnet.Chain()

	vm := fvm.NewVirtualMachine(rt)

	header := flow.Header{Height: 42}

	ctx := fvm.NewContext(
		zerolog.Nop(),
		fvm.WithChain(chain),
		fvm.WithBlockHeader(&header),
		fvm.WithCadenceLogging(true),
	)

	t.Run("works as transaction", func(t *testing.T) {
		txBody := flow.NewTransactionBody().
			SetScript([]byte(`
                transaction {
                    execute {
                        let rand = unsafeRandom()
                        log(rand)
                    }
                }
            `))

		err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
		require.NoError(t, err)

		ledger := testutil.RootBootstrappedLedger(vm, ctx)
		require.NoError(t, err)

		tx := fvm.Transaction(txBody, 0)

		err = vm.Run(ctx, tx, ledger, programs.NewEmptyPrograms())
		assert.NoError(t, err)

		assert.NoError(t, tx.Err)

		require.Len(t, tx.Logs, 1)

		num, err := strconv.ParseUint(tx.Logs[0], 10, 64)
		require.NoError(t, err)
		require.Equal(t, uint64(0xb9c618010e32a0fb), num)
	})
}

func TestBlockContext_ExecuteTransaction_CreateAccount_WithMonotonicAddresses(t *testing.T) {

	t.Parallel()

	rt := fvm.NewInterpreterRuntime()

	chain := flow.MonotonicEmulator.Chain()

	vm := fvm.NewVirtualMachine(rt)

	ctx := fvm.NewContext(
		zerolog.Nop(),
		fvm.WithChain(chain),
	)

	ledger := testutil.RootBootstrappedLedger(vm, ctx)

	txBody := flow.NewTransactionBody().
		SetScript(createAccountScript).
		AddAuthorizer(chain.ServiceAddress())

	err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
	require.NoError(t, err)

	tx := fvm.Transaction(txBody, 0)

	err = vm.Run(ctx, tx, ledger, programs.NewEmptyPrograms())
	assert.NoError(t, err)

	assert.NoError(t, tx.Err)

	accountCreatedEvents := filterAccountCreatedEvents(tx.Events)

	require.Len(t, accountCreatedEvents, 1)

	data, err := jsoncdc.Decode(accountCreatedEvents[0].Payload)
	require.NoError(t, err)
	address := flow.Address(data.(cadence.Event).Fields[0].(cadence.Address))

	assert.Equal(t, flow.HexToAddress("05"), address)
}

func TestSignatureVerification(t *testing.T) {

	t.Parallel()

	type signatureAlgorithm struct {
		name       string
		seedLength int
		algorithm  crypto.SigningAlgorithm
	}

	signatureAlgorithms := []signatureAlgorithm{
		{"ECDSA_P256", crypto.KeyGenSeedMinLenECDSAP256, crypto.ECDSAP256},
		{"ECDSA_secp256k1", crypto.KeyGenSeedMinLenECDSASecp256k1, crypto.ECDSASecp256k1},
	}

	type hashAlgorithm struct {
		name      string
		newHasher func() hash.Hasher
	}

	hashAlgorithms := []hashAlgorithm{
		{
			"SHA3_256",
			func() hash.Hasher {
				return hash.NewSHA3_256()
			},
		},
		{
			"SHA2_256",
			func() hash.Hasher {
				return hash.NewSHA2_256()
			},
		},
	}

	testForHash := func(signatureAlgorithm signatureAlgorithm, hashAlgorithm hashAlgorithm) {

		code := []byte(
			fmt.Sprintf(
				`
                      import Crypto

                      pub fun main(
                          rawPublicKeys: [[UInt8]],
                          message: [UInt8],
                          signatures: [[UInt8]],
                          weight: UFix64,
                      ): Bool {
                          let keyList = Crypto.KeyList()

                          for rawPublicKey in rawPublicKeys {
                              keyList.add(
                                  PublicKey(
                                      publicKey: rawPublicKey,
                                      signatureAlgorithm: SignatureAlgorithm.%s
                                  ),
                                  hashAlgorithm: HashAlgorithm.%s,
                                  weight: weight,
                              )
                          }

                          let signatureSet: [Crypto.KeyListSignature] = []

                          var i = 0
                          for signature in signatures {
                              signatureSet.append(
                                  Crypto.KeyListSignature(
                                      keyIndex: i,
                                      signature: signature
                                  )
                              )
                              i = i + 1
                          }

                          return keyList.verify(
                              signatureSet: signatureSet,
                              signedData: message,
                          )
                      }
                    `,
				signatureAlgorithm.name,
				hashAlgorithm.name,
			),
		)

		t.Run(fmt.Sprintf("%s %s", signatureAlgorithm.name, hashAlgorithm.name), func(t *testing.T) {

			createKey := func() (privateKey crypto.PrivateKey, publicKey cadence.Array) {
				seed := make([]byte, signatureAlgorithm.seedLength)

				var err error

				_, err = rand.Read(seed)
				require.NoError(t, err)

				privateKey, err = crypto.GeneratePrivateKey(signatureAlgorithm.algorithm, seed)
				require.NoError(t, err)

				publicKey = testutil.BytesToCadenceArray(
					privateKey.PublicKey().Encode(),
				)

				return privateKey, publicKey
			}

			createMessage := func(m string) (signableMessage []byte, message cadence.Array) {
				signableMessage = []byte(m)

				message = testutil.BytesToCadenceArray(signableMessage)

				return signableMessage, message
			}

			signMessage := func(privateKey crypto.PrivateKey, m []byte) cadence.Array {
				message := m
				if hashAlgorithm.name != "KMAC128_BLS_BLS12_381" {
					message = append(
						flow.UserDomainTag[:],
						m...,
					)
				}

				signature, err := privateKey.Sign(message, hashAlgorithm.newHasher())
				require.NoError(t, err)

				return testutil.BytesToCadenceArray(signature)
			}

			t.Run("Single key", newVMTest().run(
				func(
					t *testing.T,
					vm *fvm.VirtualMachine,
					chain flow.Chain,
					ctx fvm.Context,
					view state.View,
					programs *programs.Programs,
				) {
					privateKey, publicKey := createKey()
					signableMessage, message := createMessage("foo")
					signature := signMessage(privateKey, signableMessage)
					weight, _ := cadence.NewUFix64("1.0")

					publicKeys := cadence.NewArray([]cadence.Value{
						publicKey,
					})

					signatures := cadence.NewArray([]cadence.Value{
						signature,
					})

					t.Run("Valid", func(t *testing.T) {
						script := fvm.Script(code).WithArguments(
							jsoncdc.MustEncode(publicKeys),
							jsoncdc.MustEncode(message),
							jsoncdc.MustEncode(signatures),
							jsoncdc.MustEncode(weight),
						)

						err := vm.Run(ctx, script, view, programs)
						assert.NoError(t, err)
						assert.NoError(t, script.Err)

						assert.Equal(t, cadence.NewBool(true), script.Value)
					})

					t.Run("Invalid message", func(t *testing.T) {
						_, invalidRawMessage := createMessage("bar")

						script := fvm.Script(code).WithArguments(
							jsoncdc.MustEncode(publicKeys),
							jsoncdc.MustEncode(invalidRawMessage),
							jsoncdc.MustEncode(signatures),
							jsoncdc.MustEncode(weight),
						)

						err := vm.Run(ctx, script, view, programs)
						assert.NoError(t, err)
						assert.NoError(t, script.Err)

						assert.Equal(t, cadence.NewBool(false), script.Value)
					})

					t.Run("Invalid signature", func(t *testing.T) {
						invalidPrivateKey, _ := createKey()
						invalidRawSignature := signMessage(invalidPrivateKey, signableMessage)

						invalidRawSignatures := cadence.NewArray([]cadence.Value{
							invalidRawSignature,
						})

						script := fvm.Script(code).WithArguments(
							jsoncdc.MustEncode(publicKeys),
							jsoncdc.MustEncode(message),
							jsoncdc.MustEncode(invalidRawSignatures),
							jsoncdc.MustEncode(weight),
						)

						err := vm.Run(ctx, script, view, programs)
						assert.NoError(t, err)
						assert.NoError(t, script.Err)

						assert.Equal(t, cadence.NewBool(false), script.Value)
					})

					t.Run("Malformed public key", func(t *testing.T) {
						invalidPublicKey := testutil.BytesToCadenceArray([]byte{1, 2, 3})

						invalidPublicKeys := cadence.NewArray([]cadence.Value{
							invalidPublicKey,
						})

						script := fvm.Script(code).WithArguments(
							jsoncdc.MustEncode(invalidPublicKeys),
							jsoncdc.MustEncode(message),
							jsoncdc.MustEncode(signatures),
							jsoncdc.MustEncode(weight),
						)

						err := vm.Run(ctx, script, view, programs)
						require.NoError(t, err)
						require.Error(t, script.Err)
					})
				},
			))

			t.Run("Multiple keys", newVMTest().run(
				func(
					t *testing.T,
					vm *fvm.VirtualMachine,
					chain flow.Chain,
					ctx fvm.Context,
					view state.View,
					programs *programs.Programs,
				) {
					privateKeyA, publicKeyA := createKey()
					privateKeyB, publicKeyB := createKey()
					privateKeyC, publicKeyC := createKey()

					publicKeys := cadence.NewArray([]cadence.Value{
						publicKeyA,
						publicKeyB,
						publicKeyC,
					})

					signableMessage, message := createMessage("foo")

					signatureA := signMessage(privateKeyA, signableMessage)
					signatureB := signMessage(privateKeyB, signableMessage)
					signatureC := signMessage(privateKeyC, signableMessage)

					weight, _ := cadence.NewUFix64("0.5")

					t.Run("3 of 3", func(t *testing.T) {
						signatures := cadence.NewArray([]cadence.Value{
							signatureA,
							signatureB,
							signatureC,
						})

						script := fvm.Script(code).WithArguments(
							jsoncdc.MustEncode(publicKeys),
							jsoncdc.MustEncode(message),
							jsoncdc.MustEncode(signatures),
							jsoncdc.MustEncode(weight),
						)

						err := vm.Run(ctx, script, view, programs)
						assert.NoError(t, err)
						assert.NoError(t, script.Err)

						assert.Equal(t, cadence.NewBool(true), script.Value)
					})

					t.Run("2 of 3", func(t *testing.T) {
						signatures := cadence.NewArray([]cadence.Value{
							signatureA,
							signatureB,
						})

						script := fvm.Script(code).WithArguments(
							jsoncdc.MustEncode(publicKeys),
							jsoncdc.MustEncode(message),
							jsoncdc.MustEncode(signatures),
							jsoncdc.MustEncode(weight),
						)

						err := vm.Run(ctx, script, view, programs)
						assert.NoError(t, err)
						assert.NoError(t, script.Err)

						assert.Equal(t, cadence.NewBool(true), script.Value)
					})

					t.Run("1 of 3", func(t *testing.T) {
						signatures := cadence.NewArray([]cadence.Value{
							signatureA,
						})

						script := fvm.Script(code).WithArguments(
							jsoncdc.MustEncode(publicKeys),
							jsoncdc.MustEncode(message),
							jsoncdc.MustEncode(signatures),
							jsoncdc.MustEncode(weight),
						)

						err := vm.Run(ctx, script, view, programs)
						assert.NoError(t, err)
						assert.NoError(t, script.Err)

						assert.Equal(t, cadence.NewBool(false), script.Value)
					})
				},
			))
		})
	}

	for _, signatureAlgorithm := range signatureAlgorithms {
		for _, hashAlgorithm := range hashAlgorithms {
			testForHash(signatureAlgorithm, hashAlgorithm)
		}
	}

	testForHash(signatureAlgorithm{
		"BLS_BLS12_381",
		crypto.KeyGenSeedMinLenBLSBLS12381,
		crypto.BLSBLS12381,
	}, hashAlgorithm{
		"KMAC128_BLS_BLS12_381",
		func() hash.Hasher {
			return crypto.NewBLSKMAC(flow.UserTagString)
		},
	})
}

func TestHashing(t *testing.T) {

	t.Parallel()

	rt := fvm.NewInterpreterRuntime()

	chain := flow.Mainnet.Chain()

	vm := fvm.NewVirtualMachine(rt)

	ctx := fvm.NewContext(
		zerolog.Nop(),
		fvm.WithChain(chain),
		fvm.WithCadenceLogging(true),
	)

	ledger := testutil.RootBootstrappedLedger(vm, ctx)

	hashScript := func(hashName string) []byte {
		return []byte(fmt.Sprintf(
			`
				import Crypto
				
				pub fun main(data: [UInt8]): [UInt8] {
					return Crypto.hash(data, algorithm: HashAlgorithm.%s)
				}
			`, hashName))
	}
	hashWithTagScript := func(hashName string) []byte {
		return []byte(fmt.Sprintf(
			`
				import Crypto
				
				pub fun main(data: [UInt8], tag: String): [UInt8] {
					return Crypto.hashWithTag(data, tag: tag, algorithm: HashAlgorithm.%s)
				}
			`, hashName))
	}

	data := []byte("some random message")
	encodedBytes := make([]cadence.Value, len(data))
	for i := range encodedBytes {
		encodedBytes[i] = cadence.NewUInt8(data[i])
	}
	cadenceData := jsoncdc.MustEncode(cadence.NewArray(encodedBytes))

	// ===== Test Cases =====
	cases := []struct {
		Algo    runtime.HashAlgorithm
		WithTag bool
		Tag     string
		Check   func(t *testing.T, result string, scriptErr errors.Error, executionErr error)
	}{
		{
			Algo:    runtime.HashAlgorithmSHA2_256,
			WithTag: false,
			Check: func(t *testing.T, result string, scriptErr errors.Error, executionErr error) {
				require.NoError(t, scriptErr)
				require.NoError(t, executionErr)
				require.Equal(t, "68fb87dfba69b956f4ba98b748a75a604f99b38a4f2740290037957f7e830da8", result)
			},
		},
		{
			Algo:    runtime.HashAlgorithmSHA2_384,
			WithTag: false,
			Check: func(t *testing.T, result string, scriptErr errors.Error, executionErr error) {
				require.NoError(t, scriptErr)
				require.NoError(t, executionErr)
				require.Equal(t, "a9b3e62ab9b2a33020e015f245b82e063afd1398211326408bc8fc31c2c15859594b0aee263fbb02f6d8b5065ad49df2", result)
			},
		},
		{
			Algo:    runtime.HashAlgorithmSHA3_256,
			WithTag: false,
			Check: func(t *testing.T, result string, scriptErr errors.Error, executionErr error) {
				require.NoError(t, scriptErr)
				require.NoError(t, executionErr)
				require.Equal(t, "38effea5ab9082a2cb0dc9adfafaf88523e8f3ce74bfbeac85ffc719cc2c4677", result)
			},
		},
		{
			Algo:    runtime.HashAlgorithmSHA3_384,
			WithTag: false,
			Check: func(t *testing.T, result string, scriptErr errors.Error, executionErr error) {
				require.NoError(t, scriptErr)
				require.NoError(t, executionErr)
				require.Equal(t, "f41e8de9af0c1f46fc56d5a776f1bd500530879a85f3b904821810295927e13a54f3e936dddb84669021052eb12966c3", result)
			},
		},
		{
			Algo:    runtime.HashAlgorithmSHA2_256,
			WithTag: true,
			Tag:     "some_tag",
			Check: func(t *testing.T, result string, scriptErr errors.Error, executionErr error) {
				require.NoError(t, scriptErr)
				require.NoError(t, executionErr)
				require.Equal(t, "4e07609b9a856a5e10703d1dba73be34d9ca0f4e780859d66983f41d746ec8b2", result)
			},
		},
		{
			Algo:    runtime.HashAlgorithmSHA2_384,
			WithTag: true,
			Tag:     "some_tag",
			Check: func(t *testing.T, result string, scriptErr errors.Error, executionErr error) {
				require.NoError(t, scriptErr)
				require.NoError(t, executionErr)
				require.Equal(t, "f9bd89e15f341a225656944dc8b3c405e66a0f97838ad44c9803164c911e677aea7ad4e24486fba3f803d83ed1ccfce5", result)
			},
		},
		{
			Algo:    runtime.HashAlgorithmSHA3_256,
			WithTag: true,
			Tag:     "some_tag",
			Check: func(t *testing.T, result string, scriptErr errors.Error, executionErr error) {
				require.NoError(t, scriptErr)
				require.NoError(t, executionErr)
				require.Equal(t, "f59e2ccc9d7f008a96948a31573670d9976a4a161601ab1cd1d2da019779a0f6", result)
			},
		},
		{
			Algo:    runtime.HashAlgorithmSHA3_384,
			WithTag: true,
			Tag:     "some_tag",
			Check: func(t *testing.T, result string, scriptErr errors.Error, executionErr error) {
				require.NoError(t, scriptErr)
				require.NoError(t, executionErr)
				require.Equal(t, "e7875eafdb53327faeace8478d1650c6547d04fb4fb42f34509ad64bde0267bea7e1b3af8fda3ef9d9c9327dd4e97a96", result)
			},
		},
		{
			Algo:    runtime.HashAlgorithmKMAC128_BLS_BLS12_381,
			WithTag: false,
			Check: func(t *testing.T, result string, scriptErr errors.Error, executionErr error) {
				require.NoError(t, scriptErr)
				require.NoError(t, executionErr)
				require.Equal(t, "627d7e8fe50384601ca550ceecb61c23e9cbde7feb75ae6b53227f128f2dc3b78b543a044058403e4822f88cb7040d90d588c9e8575f0de3012fe7edaf02b9997a8a5fad234d21b2af359ec3abaeaf4a7ef60e5f04623a983bd5e071f4113678710e910d48ac4d1713073a707ab9057867e0ba32aca6b33010b1d20b8006dd25", result)
			},
		},
		{
			Algo:    runtime.HashAlgorithmKMAC128_BLS_BLS12_381,
			WithTag: true,
			Tag:     "some_tag",
			Check: func(t *testing.T, result string, scriptErr errors.Error, executionErr error) {
				require.NoError(t, scriptErr)
				require.NoError(t, executionErr)
				require.Equal(t, "dc6889f9ca46803a9c7759068989dfc3cffe632fd991e25f6589603c73b7891e2f4736eebe5248f211bbddaa3d763b1b9318185eaf3ab3bfd6f159f345c3148795e4ff3ad376c98d5616febebcf4520ca2a83dda4be2f98b1ead9fb5a622355305b156e06db173a9e1d7af973b11acc1e714cd3aa0fb367dfaadc5a957b4742b", result)
			},
		},
	}
	// ======================

	for i, c := range cases {
		t.Run(fmt.Sprintf("case %d: %s with tag: %v", i, c.Algo, c.WithTag), func(t *testing.T) {
			code := hashScript(c.Algo.Name())
			if c.WithTag {
				code = hashWithTagScript(c.Algo.Name())
			}

			script := fvm.Script(code)

			if c.WithTag {
				script = script.WithArguments(
					cadenceData,
					jsoncdc.MustEncode(cadence.String(c.Tag)),
				)
			} else {
				script = script.WithArguments(
					cadenceData,
				)
			}

			err := vm.Run(ctx, script, ledger, programs.NewEmptyPrograms())

			byteResult := make([]byte, 0)
			if err == nil && script.Err == nil {
				cadenceArray := script.Value.(cadence.Array)
				for _, value := range cadenceArray.Values {
					byteResult = append(byteResult, value.(cadence.UInt8).ToGoValue().(uint8))
				}
			}

			c.Check(t, hex.EncodeToString(byteResult), script.Err, err)
		})
	}

	hashAlgos := []runtime.HashAlgorithm{
		runtime.HashAlgorithmSHA2_256,
		runtime.HashAlgorithmSHA3_256,
		runtime.HashAlgorithmSHA2_384,
		runtime.HashAlgorithmSHA3_384,
		runtime.HashAlgorithmKMAC128_BLS_BLS12_381,
	}

	for i, algo := range hashAlgos {
		t.Run(fmt.Sprintf("compare hash results without tag %v: %v", i, algo), func(t *testing.T) {
			code := hashWithTagScript(algo.Name())
			script := fvm.Script(code)
			script = script.WithArguments(
				cadenceData,
				jsoncdc.MustEncode(cadence.String("")),
			)
			err := vm.Run(ctx, script, ledger, programs.NewEmptyPrograms())
			require.NoError(t, err)
			require.NoError(t, script.Err)

			result1 := make([]byte, 0)
			cadenceArray := script.Value.(cadence.Array)
			for _, value := range cadenceArray.Values {
				result1 = append(result1, value.(cadence.UInt8).ToGoValue().(uint8))
			}

			code = hashScript(algo.Name())
			script = fvm.Script(code)
			script = script.WithArguments(
				cadenceData,
			)
			err = vm.Run(ctx, script, ledger, programs.NewEmptyPrograms())
			require.NoError(t, err)
			require.NoError(t, script.Err)

			result2 := make([]byte, 0)
			cadenceArray = script.Value.(cadence.Array)
			for _, value := range cadenceArray.Values {
				result2 = append(result2, value.(cadence.UInt8).ToGoValue().(uint8))
			}

			result3, err := crypto2.HashWithTag(crypto2.RuntimeToCryptoHashingAlgorithm(algo), "", data)
			require.NoError(t, err)

			require.Equal(t, result1, result2)
			require.Equal(t, result1, result3)
		})
	}
}

func TestWithServiceAccount(t *testing.T) {

	t.Parallel()

	rt := fvm.NewInterpreterRuntime()

	chain := flow.Mainnet.Chain()

	vm := fvm.NewVirtualMachine(rt)

	ctxA := fvm.NewContext(
		zerolog.Nop(),
		fvm.WithChain(chain),
		fvm.WithTransactionProcessors(
			fvm.NewTransactionInvocator(zerolog.Nop()),
		),
	)

	view := utils.NewSimpleView()

	txBody := flow.NewTransactionBody().
		SetScript([]byte(`transaction { prepare(signer: AuthAccount) { AuthAccount(payer: signer) } }`)).
		AddAuthorizer(chain.ServiceAddress())

	t.Run("With service account enabled", func(t *testing.T) {
		tx := fvm.Transaction(txBody, 0)

		err := vm.Run(ctxA, tx, view, programs.NewEmptyPrograms())
		require.NoError(t, err)

		// transaction should fail on non-bootstrapped ledger
		require.Error(t, tx.Err)
	})

	t.Run("With service account disabled", func(t *testing.T) {
		ctxB := fvm.NewContextFromParent(ctxA, fvm.WithServiceAccount(false))

		tx := fvm.Transaction(txBody, 0)

		err := vm.Run(ctxB, tx, view, programs.NewEmptyPrograms())
		require.NoError(t, err)

		// transaction should succeed on non-bootstrapped ledger
		assert.NoError(t, tx.Err)
	})
}

func TestEventLimits(t *testing.T) {

	t.Parallel()

	rt := fvm.NewInterpreterRuntime()
	chain := flow.Mainnet.Chain()
	vm := fvm.NewVirtualMachine(rt)

	ctx := fvm.NewContext(
		zerolog.Nop(),
		fvm.WithChain(chain),
		fvm.WithTransactionProcessors(
			fvm.NewTransactionInvocator(zerolog.Nop()),
		),
	)

	ledger := testutil.RootBootstrappedLedger(vm, ctx)

	testContract := `
	access(all) contract TestContract {
		access(all) event LargeEvent(value: Int256, str: String, list: [UInt256], dic: {String: String})
		access(all) fun EmitEvent() {
			var s: Int256 = 1024102410241024
			var i = 0

			while i < 20 {
				emit LargeEvent(value: s, str: s.toString(), list:[], dic:{s.toString():s.toString()})
				i = i + 1
			}
		}
	}
	`

	deployingContractScriptTemplate := `
		transaction {
			prepare(signer: AuthAccount) {
				let code = "%s".decodeHex()
				signer.contracts.add(
					name: "TestContract",
					code: code
				)
		}
	}
	`

	ctx = fvm.NewContext(
		zerolog.Nop(),
		fvm.WithChain(chain),
		fvm.WithEventCollectionSizeLimit(2),
		fvm.WithTransactionProcessors(
			fvm.NewTransactionInvocator(zerolog.Nop()),
		),
	)

	txBody := flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(deployingContractScriptTemplate, hex.EncodeToString([]byte(testContract))))).
		SetPayer(chain.ServiceAddress()).
		AddAuthorizer(chain.ServiceAddress())

	programs := programs.NewEmptyPrograms()

	tx := fvm.Transaction(txBody, 0)
	err := vm.Run(ctx, tx, ledger, programs)
	require.NoError(t, err)

	txBody = flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(`
		import TestContract from 0x%s
			transaction {
			prepare(acct: AuthAccount) {}
			execute {
				TestContract.EmitEvent()
			}
		}`, chain.ServiceAddress()))).
		AddAuthorizer(chain.ServiceAddress())

	t.Run("With limits", func(t *testing.T) {
		txBody.Payer = unittest.RandomAddressFixture()
		tx := fvm.Transaction(txBody, 0)
		err := vm.Run(ctx, tx, ledger, programs)
		require.NoError(t, err)

		// transaction should fail due to event size limit
		assert.Error(t, tx.Err)
	})

	t.Run("With service account as payer", func(t *testing.T) {
		txBody.Payer = chain.ServiceAddress()
		tx := fvm.Transaction(txBody, 0)
		err := vm.Run(ctx, tx, ledger, programs)
		require.NoError(t, err)

		// transaction should not fail due to event size limit
		assert.NoError(t, tx.Err)
	})
}

func TestBlockContext_ExecuteTransaction_FailingTransactions(t *testing.T) {
	getBalance := func(vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, address flow.Address) uint64 {

		code := []byte(fmt.Sprintf(`
					import FungibleToken from 0x%s
					import FlowToken from 0x%s
					
					pub fun main(account: Address): UFix64 {
						let acct = getAccount(account)
						let vaultRef = acct.getCapability(/public/flowTokenBalance)
							.borrow<&FlowToken.Vault{FungibleToken.Balance}>()
							?? panic("Could not borrow Balance reference to the Vault")
					
						return vaultRef.balance
					}
				`, fvm.FungibleTokenAddress(chain), fvm.FlowTokenAddress(chain)))
		script := fvm.Script(code).WithArguments(
			jsoncdc.MustEncode(cadence.NewAddress(address)),
		)

		err := vm.Run(ctx, script, view, programs.NewEmptyPrograms())
		require.NoError(t, err)
		return script.Value.ToGoValue().(uint64)
	}

	t.Run("Transaction fails because of storage", newVMTest().withBootstrapProcedureOptions(
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
	).run(
		func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
			ctx.LimitAccountStorage = true // this test requires storage limits to be enforced

			// Create an account private key.
			privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
			require.NoError(t, err)

			// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
			accounts, err := testutil.CreateAccounts(vm, view, programs, privateKeys, chain)
			require.NoError(t, err)

			balanceBefore := getBalance(vm, chain, ctx, view, accounts[0])

			txBody := transferTokensTx(chain).
				AddAuthorizer(accounts[0]).
				AddArgument(jsoncdc.MustEncode(cadence.UFix64(1))).
				AddArgument(jsoncdc.MustEncode(cadence.NewAddress(chain.ServiceAddress())))

			txBody.SetProposalKey(accounts[0], 0, 0)
			txBody.SetPayer(accounts[0])

			err = testutil.SignEnvelope(txBody, accounts[0], privateKeys[0])
			require.NoError(t, err)

			tx := fvm.Transaction(txBody, 0)

			err = vm.Run(ctx, tx, view, programs)
			require.NoError(t, err)

			require.Equal(t, (&errors.StorageCapacityExceededError{}).Code(), tx.Err.Code())

			balanceAfter := getBalance(vm, chain, ctx, view, accounts[0])

			require.Equal(t, balanceAfter, balanceBefore)
		}),
	)

	t.Run("Transaction sequence number check fails and sequence number is not incremented", newVMTest().withBootstrapProcedureOptions(
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
	).
		run(
			func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
				ctx.LimitAccountStorage = true // this test requires storage limits to be enforced

				// Create an account private key.
				privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
				require.NoError(t, err)

				// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
				accounts, err := testutil.CreateAccounts(vm, view, programs, privateKeys, chain)
				require.NoError(t, err)

				txBody := transferTokensTx(chain).
					AddAuthorizer(accounts[0]).
					AddArgument(jsoncdc.MustEncode(cadence.UFix64(1_0000_0000_0000))).
					AddArgument(jsoncdc.MustEncode(cadence.NewAddress(chain.ServiceAddress())))

				// set wrong sequence number
				txBody.SetProposalKey(accounts[0], 0, 10)
				txBody.SetPayer(accounts[0])

				err = testutil.SignEnvelope(txBody, accounts[0], privateKeys[0])
				require.NoError(t, err)

				tx := fvm.Transaction(txBody, 0)

				err = vm.Run(ctx, tx, view, programs)
				require.NoError(t, err)
				require.Equal(t, (&errors.InvalidProposalSeqNumberError{}).Code(), tx.Err.Code())
				require.Equal(t, uint64(0), tx.Err.(*errors.InvalidProposalSeqNumberError).CurrentSeqNumber())
			}),
	)

	t.Run("Transaction invocation fails but sequence number is incremented", newVMTest().withBootstrapProcedureOptions(
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
	).
		run(
			func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
				ctx.LimitAccountStorage = true // this test requires storage limits to be enforced

				// Create an account private key.
				privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
				require.NoError(t, err)

				// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
				accounts, err := testutil.CreateAccounts(vm, view, programs, privateKeys, chain)
				require.NoError(t, err)

				txBody := transferTokensTx(chain).
					AddAuthorizer(accounts[0]).
					AddArgument(jsoncdc.MustEncode(cadence.UFix64(1_0000_0000_0000))).
					AddArgument(jsoncdc.MustEncode(cadence.NewAddress(chain.ServiceAddress())))

				txBody.SetProposalKey(accounts[0], 0, 0)
				txBody.SetPayer(accounts[0])

				err = testutil.SignEnvelope(txBody, accounts[0], privateKeys[0])
				require.NoError(t, err)

				tx := fvm.Transaction(txBody, 0)

				err = vm.Run(ctx, tx, view, programs)
				require.NoError(t, err)

				require.IsType(t, &errors.CadenceRuntimeError{}, tx.Err)

				// send it again
				tx = fvm.Transaction(txBody, 0)

				err = vm.Run(ctx, tx, view, programs)
				require.NoError(t, err)

				require.Equal(t, (&errors.InvalidProposalSeqNumberError{}).Code(), tx.Err.Code())
				require.Equal(t, uint64(1), tx.Err.(*errors.InvalidProposalSeqNumberError).CurrentSeqNumber())
			}),
	)
}
func TestSigningWithTags(t *testing.T) {

	checkWithTag := func(tag []byte, shouldWork bool) func(t *testing.T) {
		return newVMTest().
			run(
				func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
					// Create an account private key.
					privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
					require.NoError(t, err)

					// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
					accounts, err := testutil.CreateAccounts(vm, view, programs, privateKeys, chain)
					require.NoError(t, err)

					txBody := flow.NewTransactionBody().
						SetScript([]byte(`transaction(){}`))

					txBody.SetProposalKey(accounts[0], 0, 0)
					txBody.SetPayer(accounts[0])

					hasher, err := exeUtils.NewHasher(privateKeys[0].HashAlgo)
					require.NoError(t, err)

					sig, err := txBody.SignMessageWithTag(txBody.EnvelopeMessage(), tag, privateKeys[0].PrivateKey, hasher)
					require.NoError(t, err)
					txBody.AddEnvelopeSignature(accounts[0], 0, sig)

					tx := fvm.Transaction(txBody, 0)

					err = vm.Run(ctx, tx, view, programs)
					require.NoError(t, err)
					if shouldWork {
						require.NoError(t, tx.Err)
					} else {
						require.Error(t, tx.Err)
						require.IsType(t, tx.Err, &errors.InvalidProposalSignatureError{})
					}
				},
			)
	}

	cases := []struct {
		name      string
		tag       []byte
		shouldWok bool
	}{
		{
			name:      "no tag",
			tag:       nil,
			shouldWok: false,
		},
		{
			name:      "transaction tag",
			tag:       flow.TransactionDomainTag[:],
			shouldWok: true,
		},
		{
			name:      "user tag",
			tag:       flow.UserDomainTag[:],
			shouldWok: false,
		},
	}

	for i, c := range cases {
		works := "works"
		if !c.shouldWok {
			works = "doesn't work"
		}
		t.Run(fmt.Sprintf("Signing Transactions %d: with %s %s", i, c.name, works), checkWithTag(c.tag, c.shouldWok))
	}

}

func TestTransactionFeeDeduction(t *testing.T) {
	getBalance := func(vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, address flow.Address) uint64 {

		code := []byte(fmt.Sprintf(`
					import FungibleToken from 0x%s
					import FlowToken from 0x%s
					
					pub fun main(account: Address): UFix64 {
						let acct = getAccount(account)
						let vaultRef = acct.getCapability(/public/flowTokenBalance)
							.borrow<&FlowToken.Vault{FungibleToken.Balance}>()
							?? panic("Could not borrow Balance reference to the Vault")
					
						return vaultRef.balance
					}
				`, fvm.FungibleTokenAddress(chain), fvm.FlowTokenAddress(chain)))
		script := fvm.Script(code).WithArguments(
			jsoncdc.MustEncode(cadence.NewAddress(address)),
		)

		err := vm.Run(ctx, script, view, programs.NewEmptyPrograms())
		require.NoError(t, err)
		return script.Value.ToGoValue().(uint64)
	}

	type testCase struct {
		name          string
		fundWith      uint64
		tryToTransfer uint64
		checkResult   func(t *testing.T, balanceBefore uint64, balanceAfter uint64, tx *fvm.TransactionProcedure)
	}

	txFees := fvm.DefaultTransactionFees.ToGoValue().(uint64)
	fundingAmount := uint64(1_0000_0000)
	transferAmount := uint64(123_456)
	minimumStorageReservation := fvm.DefaultMinimumStorageReservation.ToGoValue().(uint64)

	testCases := []testCase{
		{
			name:          "Transaction fees are deducted",
			fundWith:      fundingAmount,
			tryToTransfer: 0,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, tx *fvm.TransactionProcedure) {
				require.NoError(t, tx.Err)
				require.Equal(t, txFees, balanceBefore-balanceAfter)
			},
		},
		{
			name:          "Transaction fee deduction emits events",
			fundWith:      fundingAmount,
			tryToTransfer: 0,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, tx *fvm.TransactionProcedure) {
				require.NoError(t, tx.Err)

				var deposits []flow.Event
				var withdraws []flow.Event

				for _, e := range tx.Events {
					if string(e.Type) == fmt.Sprintf("A.%s.FlowToken.TokensDeposited", fvm.FlowTokenAddress(flow.Testnet.Chain())) {
						deposits = append(deposits, e)
					}
					if string(e.Type) == fmt.Sprintf("A.%s.FlowToken.TokensWithdrawn", fvm.FlowTokenAddress(flow.Testnet.Chain())) {
						withdraws = append(withdraws, e)
					}
				}

				require.Len(t, deposits, 2)
				require.Len(t, withdraws, 2)
			},
		},
		{
			name:          "Transaction fees are deducted and tx is applied",
			fundWith:      fundingAmount,
			tryToTransfer: transferAmount,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, tx *fvm.TransactionProcedure) {
				require.NoError(t, tx.Err)
				require.Equal(t, txFees+transferAmount, balanceBefore-balanceAfter)
			},
		},
		{
			name:          "If just enough balance, fees are deducted",
			fundWith:      txFees + transferAmount,
			tryToTransfer: transferAmount,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, tx *fvm.TransactionProcedure) {
				require.NoError(t, tx.Err)
				require.Equal(t, uint64(0), balanceAfter)
			},
		},
		{
			// this is an edge case that is not applicable to any network.
			// If storage limits were on this would fail due to storage limits
			name:          "If not enough balance, transaction succeeds and fees are deducted to 0",
			fundWith:      txFees,
			tryToTransfer: 1,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, tx *fvm.TransactionProcedure) {
				require.NoError(t, tx.Err)
				require.Equal(t, uint64(0), balanceAfter)
			},
		},
		{
			name:          "If tx fails, fees are deducted",
			fundWith:      fundingAmount,
			tryToTransfer: 2 * fundingAmount,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, tx *fvm.TransactionProcedure) {
				require.Error(t, tx.Err)
				require.Equal(t, fundingAmount-txFees, balanceAfter)
			},
		},
		{
			name:          "If tx fails, fee deduction events are emitted",
			fundWith:      fundingAmount,
			tryToTransfer: 2 * fundingAmount,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, tx *fvm.TransactionProcedure) {
				require.Error(t, tx.Err)

				var deposits []flow.Event
				var withdraws []flow.Event

				for _, e := range tx.Events {
					if string(e.Type) == fmt.Sprintf("A.%s.FlowToken.TokensDeposited", fvm.FlowTokenAddress(flow.Testnet.Chain())) {
						deposits = append(deposits, e)
					}
					if string(e.Type) == fmt.Sprintf("A.%s.FlowToken.TokensWithdrawn", fvm.FlowTokenAddress(flow.Testnet.Chain())) {
						withdraws = append(withdraws, e)
					}
				}

				require.Len(t, deposits, 1)
				require.Len(t, withdraws, 1)
			},
		},
	}

	testCasesWithStorageEnabled := []testCase{
		{
			name:          "Transaction fees are deducted",
			fundWith:      fundingAmount,
			tryToTransfer: 0,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, tx *fvm.TransactionProcedure) {
				require.NoError(t, tx.Err)
				require.Equal(t, txFees, balanceBefore-balanceAfter)
			},
		},
		{
			name:          "Transaction fee deduction emits events",
			fundWith:      fundingAmount,
			tryToTransfer: 0,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, tx *fvm.TransactionProcedure) {
				require.NoError(t, tx.Err)

				var deposits []flow.Event
				var withdraws []flow.Event

				for _, e := range tx.Events {
					if string(e.Type) == fmt.Sprintf("A.%s.FlowToken.TokensDeposited", fvm.FlowTokenAddress(flow.Testnet.Chain())) {
						deposits = append(deposits, e)
					}
					if string(e.Type) == fmt.Sprintf("A.%s.FlowToken.TokensWithdrawn", fvm.FlowTokenAddress(flow.Testnet.Chain())) {
						withdraws = append(withdraws, e)
					}
				}

				require.Len(t, deposits, 2)
				require.Len(t, withdraws, 2)
			},
		},
		{
			name:          "Transaction fees are deducted and tx is applied",
			fundWith:      fundingAmount,
			tryToTransfer: transferAmount,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, tx *fvm.TransactionProcedure) {
				require.NoError(t, tx.Err)
				require.Equal(t, txFees+transferAmount, balanceBefore-balanceAfter)
			},
		},
		{
			name:          "If just enough balance, fees are deducted",
			fundWith:      txFees + transferAmount,
			tryToTransfer: transferAmount,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, tx *fvm.TransactionProcedure) {
				require.NoError(t, tx.Err)
				require.Equal(t, minimumStorageReservation, balanceAfter)
			},
		},
		{
			name:          "If tx fails, fees are deducted",
			fundWith:      fundingAmount,
			tryToTransfer: 2 * fundingAmount,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, tx *fvm.TransactionProcedure) {
				require.Error(t, tx.Err)
				require.Equal(t, fundingAmount-txFees+minimumStorageReservation, balanceAfter)
			},
		},
		{
			name:          "If tx fails, fee deduction events are emitted",
			fundWith:      fundingAmount,
			tryToTransfer: 2 * fundingAmount,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, tx *fvm.TransactionProcedure) {
				require.Error(t, tx.Err)

				var deposits []flow.Event
				var withdraws []flow.Event

				for _, e := range tx.Events {
					if string(e.Type) == fmt.Sprintf("A.%s.FlowToken.TokensDeposited", fvm.FlowTokenAddress(flow.Testnet.Chain())) {
						deposits = append(deposits, e)
					}
					if string(e.Type) == fmt.Sprintf("A.%s.FlowToken.TokensWithdrawn", fvm.FlowTokenAddress(flow.Testnet.Chain())) {
						withdraws = append(withdraws, e)
					}
				}

				require.Len(t, deposits, 1)
				require.Len(t, withdraws, 1)
			},
		},
		{
			name:          "If balance at minimum, transaction fails, fees are deducted and fee deduction events are emitted",
			fundWith:      0,
			tryToTransfer: 0,
			checkResult: func(t *testing.T, balanceBefore uint64, balanceAfter uint64, tx *fvm.TransactionProcedure) {
				require.Error(t, tx.Err)
				require.Equal(t, minimumStorageReservation-txFees, balanceAfter)

				var deposits []flow.Event
				var withdraws []flow.Event

				for _, e := range tx.Events {
					if string(e.Type) == fmt.Sprintf("A.%s.FlowToken.TokensDeposited", fvm.FlowTokenAddress(flow.Testnet.Chain())) {
						deposits = append(deposits, e)
					}
					if string(e.Type) == fmt.Sprintf("A.%s.FlowToken.TokensWithdrawn", fvm.FlowTokenAddress(flow.Testnet.Chain())) {
						withdraws = append(withdraws, e)
					}
				}

				require.Len(t, deposits, 1)
				require.Len(t, withdraws, 1)
			},
		},
	}

	runTx := func(tc testCase) func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
		return func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
			// ==== Create an account ====
			privateKey, txBody := testutil.CreateAccountCreationTransaction(t, chain)

			err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
			require.NoError(t, err)

			tx := fvm.Transaction(txBody, 0)

			err = vm.Run(ctx, tx, view, programs)
			require.NoError(t, err)

			assert.NoError(t, tx.Err)

			assert.Len(t, tx.Events, 10)

			accountCreatedEvents := filterAccountCreatedEvents(tx.Events)

			require.Len(t, accountCreatedEvents, 1)

			// read the address of the account created (e.g. "0x01" and convert it to flow.address)
			data, err := jsoncdc.Decode(accountCreatedEvents[0].Payload)
			require.NoError(t, err)
			address := flow.Address(data.(cadence.Event).Fields[0].(cadence.Address))

			// ==== Transfer tokens to new account ====
			txBody = transferTokensTx(chain).
				AddAuthorizer(chain.ServiceAddress()).
				AddArgument(jsoncdc.MustEncode(cadence.UFix64(tc.fundWith))).
				AddArgument(jsoncdc.MustEncode(cadence.NewAddress(address)))

			txBody.SetProposalKey(chain.ServiceAddress(), 0, 1)
			txBody.SetPayer(chain.ServiceAddress())

			err = testutil.SignEnvelope(
				txBody,
				chain.ServiceAddress(),
				unittest.ServiceAccountPrivateKey,
			)
			require.NoError(t, err)

			tx = fvm.Transaction(txBody, 0)

			err = vm.Run(ctx, tx, view, programs)
			require.NoError(t, err)
			require.NoError(t, tx.Err)

			balanceBefore := getBalance(vm, chain, ctx, view, address)

			// ==== Transfer tokens from new account ====

			txBody = transferTokensTx(chain).
				AddAuthorizer(address).
				AddArgument(jsoncdc.MustEncode(cadence.UFix64(tc.tryToTransfer))).
				AddArgument(jsoncdc.MustEncode(cadence.NewAddress(chain.ServiceAddress())))

			txBody.SetProposalKey(address, 0, 0)
			txBody.SetPayer(address)

			err = testutil.SignEnvelope(
				txBody,
				address,
				privateKey,
			)
			require.NoError(t, err)

			tx = fvm.Transaction(txBody, 1)

			err = vm.Run(ctx, tx, view, programs)
			require.NoError(t, err)

			balanceAfter := getBalance(vm, chain, ctx, view, address)

			tc.checkResult(
				t,
				balanceBefore,
				balanceAfter,
				tx,
			)
		}
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("Transaction Fees %d: %s", i, tc.name), newVMTest().withBootstrapProcedureOptions(
			fvm.WithTransactionFee(fvm.DefaultTransactionFees),
		).withContextOptions(
			fvm.WithTransactionFeesEnabled(true),
		).run(
			runTx(tc)),
		)
	}

	for i, tc := range testCasesWithStorageEnabled {
		t.Run(fmt.Sprintf("Transaction Fees with storage %d: %s", i, tc.name), newVMTest().withBootstrapProcedureOptions(
			fvm.WithTransactionFee(fvm.DefaultTransactionFees),
			fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
			fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
			fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		).withContextOptions(
			fvm.WithTransactionFeesEnabled(true),
			fvm.WithAccountStorageLimit(true),
		).run(
			runTx(tc)),
		)
	}
}

func TestStorageUsed(t *testing.T) {
	t.Parallel()

	rt := fvm.NewInterpreterRuntime()

	chain := flow.Testnet.Chain()

	vm := fvm.NewVirtualMachine(rt)

	ctx := fvm.NewContext(
		zerolog.Nop(),
		fvm.WithChain(chain),
		fvm.WithCadenceLogging(true),
	)

	code := []byte(`
        pub fun main() {

            var addresses: [Address]= [
                0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731,
                0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731,
                0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731,
                0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731,
                0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731,
                0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731,
                0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731,
                0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731,
                0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731,
                0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731,
                0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731, 0x2a3c4c2581cef731
            ]

            var count = 0
            for address in addresses {
                let account = getAccount(address)
                var x = account.storageUsed
            }
        }
	`)

	address, err := hex.DecodeString("2a3c4c2581cef731")
	require.NoError(t, err)

	storageUsed := make([]byte, 8)
	binary.BigEndian.PutUint64(storageUsed, 0)

	simpleView := utils.NewSimpleView()
	simpleView.Set(string(address), "", state.KeyStorageUsed, storageUsed)

	script := fvm.Script(code)

	err = vm.Run(ctx, script, simpleView, programs.NewEmptyPrograms())
	assert.NoError(t, err)
}
