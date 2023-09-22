package util

import (
	"bytes"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

type PayloadAccountGroup struct {
	Address  common.Address
	Payloads []*ledger.Payload
}

type PayloadAccountGrouping struct {
	payloads sortablePayloads
	indexes  []int

	current int
}

func (g *PayloadAccountGrouping) Next() (*PayloadAccountGroup, error) {
	if g.current == len(g.indexes) {
		// reached the end
		return nil, nil
	}
	accountStartIndex := g.indexes[g.current]
	accountEndIndex := len(g.payloads)
	if g.current != len(g.indexes)-1 {
		accountEndIndex = g.indexes[g.current+1]
	}

	address, err := payloadToAddress(g.payloads[accountStartIndex])
	if err != nil {
		return nil, fmt.Errorf("failed to get address from payload: %w", err)
	}

	g.current++

	return &PayloadAccountGroup{
		Address:  address,
		Payloads: g.payloads[accountStartIndex:accountEndIndex],
	}, nil
}

func (g *PayloadAccountGrouping) Len() int {
	return len(g.indexes)
}

func GroupPayloadsByAccount(log zerolog.Logger, payloads []*ledger.Payload, nWorkers int) *PayloadAccountGrouping {
	if len(payloads) == 0 {
		return &PayloadAccountGrouping{}
	}
	p := sortablePayloads(payloads)

	start := time.Now()
	log.Info().
		Int("payloads", len(payloads)).
		Int("workers", nWorkers).
		Msg("Sorting payloads by address")

	// sort the payloads by address
	sortPayloads(0, len(p), p, make(sortablePayloads, len(p)), nWorkers)
	end := time.Now()

	log.Info().
		Int("payloads", len(payloads)).
		Str("duration", end.Sub(start).Round(1*time.Second).String()).
		Msg("Sorted. Finding account boundaries in sorted payloads")

	start = time.Now()
	// find the indexes of the payloads that start a new account
	indexes := make([]int, 0, len(p))
	for i := 0; i < len(p); {
		indexes = append(indexes, i)
		i = p.FindLastOfTheSameKey(i) + 1
	}
	end = time.Now()

	log.Info().
		Int("accounts", len(indexes)).
		Str("duration", end.Sub(start).Round(1*time.Second).String()).
		Msg("Done grouping payloads by account")

	return &PayloadAccountGrouping{
		payloads: p,
		indexes:  indexes,
	}
}

// payloadToAddress takes a payload and return:
// - (address, true, nil) if the payload is for an account, the account address is returned
// - ("", false, nil) if the payload is not for an account
// - ("", false, err) if running into any exception
// The zero address is used for global Payloads and is not an account
func payloadToAddress(p *ledger.Payload) (common.Address, error) {
	k, err := p.Key()
	if err != nil {
		return common.ZeroAddress, fmt.Errorf("could not find key for payload: %w", err)
	}

	id, err := KeyToRegisterID(k)
	if err != nil {
		return common.ZeroAddress, fmt.Errorf("error converting key to register ID")
	}

	if len([]byte(id.Owner)) != flow.AddressLength {
		return common.ZeroAddress, nil
	}

	address, err := common.BytesToAddress([]byte(id.Owner))
	if err != nil {
		return common.ZeroAddress, fmt.Errorf("invalid account address: %w", err)
	}

	return address, nil
}

// EncodedKeyAddressPrefixLength is the length of the address prefix in the encoded key
// 2 for uint16 of number of key parts
// 4 for uint32 of the length of the first key part
// 2 for uint32 of the key part type
// 8 for the address which is the actual length of the first key part
const EncodedKeyAddressPrefixLength = 2 + 4 + 2 + flow.AddressLength

type sortablePayloads []*ledger.Payload

func (s sortablePayloads) Len() int {
	return len(s)
}

func (s sortablePayloads) Less(i, j int) bool {
	return s.Compare(i, j) < 0
}

func (s sortablePayloads) Compare(i, j int) int {
	return bytes.Compare(
		s[i].EncodedKey()[:EncodedKeyAddressPrefixLength],
		s[j].EncodedKey()[:EncodedKeyAddressPrefixLength],
	)
}

func (s sortablePayloads) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s sortablePayloads) FindLastOfTheSameKey(i int) int {
	low := i
	step := 1
	for low+step < len(s) && s.Compare(low+step, i) == 0 {
		low += step
		step *= 2
	}

	high := low + step
	if high > len(s) {
		high = len(s)
	}

	for low < high {
		mid := (low + high) / 2
		if s.Compare(mid, i) == 0 {
			low = mid + 1
		} else {
			high = mid
		}
	}

	return low - 1
}

// sortPayloads sorts the payloads in the range [i, j) using goroutines and merges
// the results using the intermediate buffer. The goroutine allowance is the number
// of goroutines that can be used for sorting. If the allowance is less than 2,
// the payloads are sorted using the built-in sort.
// The buffer must be of the same length as the source and can be disposed after.
func sortPayloads(i, j int, source, buffer sortablePayloads, goroutineAllowance int) {
	// if the length is less than 2, no need to sort
	if j-i <= 1 {
		return
	}

	// below this size, no need to split the sorting into goroutines
	const minSizeForSplit = 100_000

	// if we are out of goroutine allowance, sort with built-in sortß
	// if the length is less than minSizeForSplit, sort with built-in sort
	if goroutineAllowance < 2 || j-i < minSizeForSplit {
		sort.Sort(source[i:j])
		return
	}

	goroutineAllowance -= 2
	allowance1 := goroutineAllowance / 2
	allowance2 := goroutineAllowance - allowance1
	mid := (i + j) / 2

	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		sortPayloads(i, mid, source, buffer, allowance1)
		wg.Done()
	}()
	go func() {
		sortPayloads(mid, j, source, buffer, allowance2)
		wg.Done()
	}()
	wg.Wait()

	mergeInto(source, buffer, i, mid, j)
}

func mergeInto(source, buffer sortablePayloads, i int, mid int, j int) {
	left := i
	right := mid
	for k := i; k < j; k++ {
		if left < mid && (right >= j || source.Compare(left, right) <= 0) {
			buffer[k] = source[left]
			left++
		} else {
			buffer[k] = source[right]
			right++
		}
	}

	for k := i; k < j; k++ {
		source[k] = buffer[k]
	}
}