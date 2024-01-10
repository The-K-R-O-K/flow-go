package precompiles

import (
	"fmt"

	"github.com/onflow/flow-go/fvm/evm/types"
)

// This is derived as the first 4 bytes of the Keccak hash of the ASCII form of the signature of the method
type MethodID [4]byte

type Callable interface {
	MethodID() MethodID

	ComputeGas(input []byte) uint64

	Run(input []byte) ([]byte, error)
}

func multiMethodPrecompileContract(
	address types.Address,
	callables map[MethodID]Callable,
) types.Precompile {
	return &Precompile{
		callables: callables,
		address:   address,
	}
}

type Precompile struct {
	callables map[MethodID]Callable
	address   types.Address
}

func (p *Precompile) Address() types.Address {
	return p.address
}

// RequiredPrice calculates the contract gas use
func (p *Precompile) RequiredGas(input []byte) uint64 {
	if len(input) < 4 {
		return 0
	}
	mID, data := splitMethodID(input)
	callable, found := p.callables[mID]
	if !found {
		return 0
	}
	return callable.ComputeGas(data)
}

// Run runs the precompiled contract
func (p *Precompile) Run(input []byte) ([]byte, error) {
	if len(input) < 4 {
		return nil, fmt.Errorf("invalid method") // TODO return the right error based on geth
	}
	mID, data := splitMethodID(input)
	callable, found := p.callables[mID]
	if !found {
		return nil, fmt.Errorf("invalid method") // TODO return the right error based on geth
	}
	return callable.Run(data)
}

func splitMethodID(input []byte) (MethodID, []byte) {
	var methodID MethodID
	copy(methodID[:], input[0:4])
	return methodID, input[4:]
}
