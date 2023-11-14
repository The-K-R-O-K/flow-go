package stdlib

import (
	_ "embed"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/errors"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/cadence/runtime/stdlib"

	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

//go:embed contract.cdc
var ContractCode []byte

const ContractName = "EVM"

var EVMTransactionBytesCadenceType = cadence.NewVariableSizedArrayType(cadence.TheUInt8Type)
var evmTransactionBytesType = sema.NewVariableSizedType(nil, sema.UInt8Type)

var evmAddressBytesType = sema.NewConstantSizedType(nil, sema.UInt8Type, types.AddressLength)
var evmAddressBytesStaticType = interpreter.ConvertSemaArrayTypeToStaticArrayType(nil, evmAddressBytesType)
var EVMAddressBytesCadenceType = cadence.NewConstantSizedArrayType(types.AddressLength, cadence.TheUInt8Type)

const internalEVMTypeRunFunctionName = "run"

var internalEVMTypeRunFunctionType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Label:          "tx",
			TypeAnnotation: sema.NewTypeAnnotation(evmTransactionBytesType),
		},
		{
			Label:          "coinbase",
			TypeAnnotation: sema.NewTypeAnnotation(evmAddressBytesType),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.BoolType),
}

func newInternalEVMTypeRunFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewHostFunctionValue(
		gauge,
		internalEVMTypeRunFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			inter := invocation.Interpreter
			locationRange := invocation.LocationRange

			// Get transaction argument

			transactionValue, ok := invocation.Arguments[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			transaction, err := interpreter.ByteArrayValueToByteSlice(inter, transactionValue, locationRange)
			if err != nil {
				panic(err)
			}

			// Get coinbase argument

			coinbaseValue, ok := invocation.Arguments[1].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			coinbase, err := interpreter.ByteArrayValueToByteSlice(inter, coinbaseValue, locationRange)
			if err != nil {
				panic(err)
			}

			// Run

			cb := types.NewAddressFromBytes(coinbase)
			handler.Run(transaction, cb)

			return interpreter.Void
		},
	)
}

func EVMAddressToAddressBytesArrayValue(
	inter *interpreter.Interpreter,
	address types.Address,
) *interpreter.ArrayValue {
	var index int
	return interpreter.NewArrayValueWithIterator(
		inter,
		evmAddressBytesStaticType,
		common.ZeroAddress,
		types.AddressLength,
		func() interpreter.Value {
			if index >= types.AddressLength {
				return nil
			}
			result := interpreter.NewUInt8Value(inter, func() uint8 {
				return address[index]
			})
			index++
			return result
		},
	)
}

const internalEVMTypeCallFunctionName = "call"

var internalEVMTypeCallFunctionType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Label:          "from",
			TypeAnnotation: sema.NewTypeAnnotation(evmAddressBytesType),
		},
		{
			Label:          "to",
			TypeAnnotation: sema.NewTypeAnnotation(evmAddressBytesType),
		},
		{
			Label:          "data",
			TypeAnnotation: sema.NewTypeAnnotation(sema.ByteArrayType),
		},
		{
			Label:          "gasLimit",
			TypeAnnotation: sema.NewTypeAnnotation(sema.UInt64Type),
		},
		{
			Label:          "value",
			TypeAnnotation: sema.NewTypeAnnotation(sema.UFix64Type),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.ByteArrayType),
}

func AddressBytesArrayValueToEVMAddress(
	inter *interpreter.Interpreter,
	locationRange interpreter.LocationRange,
	addressBytesValue *interpreter.ArrayValue,
) (
	result types.Address,
	err error,
) {
	// Convert

	var bytes []byte
	bytes, err = interpreter.ByteArrayValueToByteSlice(
		inter,
		addressBytesValue,
		locationRange,
	)
	if err != nil {
		return result, err
	}

	// Check length

	length := len(bytes)
	const expectedLength = types.AddressLength
	if length != expectedLength {
		return result, errors.NewDefaultUserError(
			"invalid address length: got %d, expected %d",
			length,
			expectedLength,
		)
	}

	copy(result[:], bytes)

	return result, nil
}

func newInternalEVMTypeCallFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewHostFunctionValue(
		gauge,
		internalEVMTypeCallFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			inter := invocation.Interpreter
			locationRange := invocation.LocationRange

			// Get from address

			fromAddressValue, ok := invocation.Arguments[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			fromAddress, err := AddressBytesArrayValueToEVMAddress(inter, locationRange, fromAddressValue)
			if err != nil {
				panic(err)
			}

			// Get to address

			toAddressValue, ok := invocation.Arguments[1].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			toAddress, err := AddressBytesArrayValueToEVMAddress(inter, locationRange, toAddressValue)
			if err != nil {
				panic(err)
			}

			// Get data

			dataValue, ok := invocation.Arguments[2].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			data, err := interpreter.ByteArrayValueToByteSlice(inter, dataValue, locationRange)
			if err != nil {
				panic(err)
			}

			// Get gas limit

			gasLimitValue, ok := invocation.Arguments[3].(interpreter.UInt64Value)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			gasLimit := types.GasLimit(gasLimitValue)

			// Get balance

			balanceValue, ok := invocation.Arguments[4].(interpreter.UFix64Value)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			balance := types.Balance(balanceValue)

			// Call

			const isAuthorized = true
			account := handler.AccountByAddress(fromAddress, isAuthorized)
			result := account.Call(toAddress, data, gasLimit, balance)

			return interpreter.ByteSliceToByteArrayValue(inter, result)
		},
	)
}

const internalEVMTypeCreateBridgedAccountFunctionName = "createBridgedAccount"

var internalEVMTypeCreateBridgedAccountFunctionType = &sema.FunctionType{
	ReturnTypeAnnotation: sema.NewTypeAnnotation(evmAddressBytesType),
}

func newInternalEVMTypeCreateBridgedAccountFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewHostFunctionValue(
		gauge,
		internalEVMTypeCreateBridgedAccountFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			inter := invocation.Interpreter
			address := handler.AllocateAddress()
			return EVMAddressToAddressBytesArrayValue(inter, address)
		},
	)
}

func NewInternalEVMContractValue(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.SimpleCompositeValue {
	return interpreter.NewSimpleCompositeValue(
		gauge,
		InternalEVMContractType.ID(),
		internalEVMContractStaticType,
		InternalEVMContractType.Fields,
		map[string]interpreter.Value{
			internalEVMTypeRunFunctionName:                  newInternalEVMTypeRunFunction(gauge, handler),
			internalEVMTypeCreateBridgedAccountFunctionName: newInternalEVMTypeCreateBridgedAccountFunction(gauge, handler),
			internalEVMTypeCallFunctionName:                 newInternalEVMTypeCallFunction(gauge, handler),
		},
		nil,
		nil,
		nil,
	)
}

const InternalEVMContractName = "InternalEVM"

var InternalEVMContractType = func() *sema.CompositeType {
	ty := &sema.CompositeType{
		Identifier: InternalEVMContractName,
		Kind:       common.CompositeKindContract,
	}

	ty.Members = sema.MembersAsMap([]*sema.Member{
		sema.NewUnmeteredPublicFunctionMember(
			ty,
			internalEVMTypeRunFunctionName,
			internalEVMTypeRunFunctionType,
			"",
		),
		sema.NewUnmeteredPublicFunctionMember(
			ty,
			internalEVMTypeCreateBridgedAccountFunctionName,
			internalEVMTypeCreateBridgedAccountFunctionType,
			"",
		),
		sema.NewUnmeteredPublicFunctionMember(
			ty,
			internalEVMTypeCallFunctionName,
			internalEVMTypeCallFunctionType,
			"",
		),
	})
	return ty
}()

var internalEVMContractStaticType = interpreter.ConvertSemaCompositeTypeToStaticCompositeType(
	nil,
	InternalEVMContractType,
)

func newInternalEVMStandardLibraryValue(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) stdlib.StandardLibraryValue {
	return stdlib.StandardLibraryValue{
		Name:  InternalEVMContractName,
		Type:  InternalEVMContractType,
		Value: NewInternalEVMContractValue(gauge, handler),
		Kind:  common.DeclarationKindContract,
	}
}

var internalEVMStandardLibraryType = stdlib.StandardLibraryType{
	Name: InternalEVMContractName,
	Type: InternalEVMContractType,
	Kind: common.DeclarationKindContract,
}

func SetupEnvironment(env runtime.Environment, handler types.ContractHandler, service flow.Address) {
	location := common.NewAddressLocation(nil, common.Address(service), ContractName)
	env.DeclareType(
		internalEVMStandardLibraryType,
		location,
	)
	env.DeclareValue(
		newInternalEVMStandardLibraryValue(nil, handler),
		location,
	)
}

func NewEVMAddressCadenceType(address common.Address) *cadence.StructType {
	return cadence.NewStructType(
		common.NewAddressLocation(nil, address, ContractName),
		"EVM.EVMAddress",
		[]cadence.Field{
			{
				Identifier: "bytes",
				Type:       EVMAddressBytesCadenceType,
			},
		},
		nil,
	)
}

func NewBalanceCadenceType(address common.Address) *cadence.StructType {
	return cadence.NewStructType(
		common.NewAddressLocation(nil, address, ContractName),
		"EVM.Balance",
		[]cadence.Field{
			{
				Identifier: "flow",
				Type:       cadence.UFix64Type{},
			},
		},
		nil,
	)
}
