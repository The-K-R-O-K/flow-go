package crypto

// #cgo CFLAGS: -g -Wall -std=c99 -I./ -I./relic/include -I./relic/include/low
// #cgo LDFLAGS: -Lrelic/build/lib -l relic_s
// #include "dkg_include.h"
import "C"
import (
	log "github.com/sirupsen/logrus"
)

func (s *feldmanVSSQualState) receiveShare(origin index, data []byte) (DKGresult, []DKGToSend, error) {
	// only accept private shares from the leader.
	if origin != s.leaderIndex {
		return invalid, nil, nil
	}

	if s.xReceived {
		return invalid, nil, nil
	}
	if (len(data)) != PrKeyLengthBLS_BLS12381 {
		return invalid, nil, nil
	}
	// temporary log
	log.Debugf("%d Receiving a share from %d\n", s.currentIndex, origin)
	log.Debugf("the share is %d\n", data)
	// read the node private share
	C.bn_read_bin((*C.bn_st)(&s.x),
		(*C.uchar)(&data[0]),
		PrKeyLengthBLS_BLS12381,
	)

	s.xReceived = true
	if s.AReceived {
		result := s.verifyShare()
		if result == valid {
			return result, nil, nil
		}
		// otherwise, build a complaint to send and add it to the local
		// complaints map
		toSend := DKGToSend{
			broadcast: true,
			data:      []byte{byte(FeldmanVSSComplaint), byte(s.leaderIndex)},
		}
		s.complaints[s.currentIndex] = &complaint{
			received:       true,
			answerReceived: false,
		}
		return result, []DKGToSend{toSend}, nil
	}
	return valid, nil, nil
}

func (s *feldmanVSSQualState) receiveVerifVector(origin index, data []byte) (DKGresult, []DKGToSend, error) {
	// only accept the verification vector from the leader.
	if origin != s.leaderIndex {
		return invalid, nil, nil
	}

	if s.AReceived {
		return invalid, nil, nil
	}
	if (PubKeyLengthBLS_BLS12381)*(s.threshold+1) != len(data) {
		return invalid, nil, nil
	}

	// temporary log
	log.Debugf("%d Receiving vector from %d\n", s.currentIndex, origin)
	log.Debugf("the vector is %d\n", data)

	// read the verification vector
	s.A = make([]pointG2, s.threshold+1)
	readVerifVector(s.A, data)

	s.y = make([]pointG2, s.size)
	s.computePublicKeys()

	s.AReceived = true
	// check the (already) registered complaints
	for complainee, c := range s.complaints {
		if c.received && c.answerReceived && !c.validComplaint {
			s.checkComplaint(complainee, c)
		}
	}
	// check the private share
	if s.xReceived {
		result := s.verifyShare()
		if result == valid {
			return result, nil, nil
		}
		// otherwise, build a complaint to send and add it to the local
		// complaints map
		toSend := DKGToSend{
			broadcast: true,
			data:      []byte{byte(FeldmanVSSComplaint), byte(s.leaderIndex)},
		}
		s.complaints[s.currentIndex] = &complaint{
			received:       true,
			answerReceived: false,
		}
		return result, []DKGToSend{toSend}, nil
	}
	return valid, nil, nil
}

// assuming a complaint and its answer were received, this function updates
// validComplaint:
// - false if the answer is valid
// - true if the complaint is valid
func (s *feldmanVSSQualState) checkComplaint(complainee index, c *complaint) {
	// check y[complainee] == share.G2
	c.validComplaint = C.verifyshare((*C.bn_st)(&c.answer),
		(*C.ep2_st)(&s.y[complainee])) == 0
}

// data = |complainee|
func (s *feldmanVSSQualState) receiveComplaint(origin index, data []byte) (DKGresult, []DKGToSend, error) {
	// first byte encodes the complainee
	complainee := index(data[0])

	// if the complainee is not the leader, ignore the complaint
	if complainee != s.leaderIndex || len(data) != 1 {
		return invalid, nil, nil
	}

	c, ok := s.complaints[origin]
	// if the complaint is new, add it
	if !ok {
		s.complaints[origin] = &complaint{
			received:       true,
			answerReceived: false,
		}
		// if the complainee is the current node, prepare an answer
		if s.currentIndex == s.leaderIndex {
			complainAnswerSize := 2 + PrKeyLengthBLS_BLS12381
			data := make([]byte, complainAnswerSize)
			data[0] = byte(FeldmanVSSComplaintAnswer)
			data[1] = byte(origin)
			ZrPolynomialImage(data[2:], s.a, origin, nil)
			toSend := DKGToSend{
				broadcast: true,
				data:      data,
			}
			s.complaints[origin].answerReceived = true
			s.complaints[origin].validComplaint = false
			return valid, []DKGToSend{toSend}, nil
		}
		return valid, nil, nil
	}
	// complaint is not new in the map
	// check if the complain has been already received
	if c.received {
		return invalid, nil, nil
	}
	c.received = true
	// first flag check is a sanity check
	if c.answerReceived && !c.validComplaint && s.currentIndex != s.leaderIndex {
		s.checkComplaint(origin, c)
		return valid, nil, nil
	}
	return invalid, nil, nil
}

// answer = |complainer| private share |
func (s *feldmanVSSQualState) receiveComplaintAnswer(origin index, data []byte) (DKGresult, error) {

	// first byte encodes the complainee
	complainer := index(data[0])

	// check for invalid answers
	complainAnswerSize := 2 + PrKeyLengthBLS_BLS12381
	if origin != s.leaderIndex {
		return invalid, nil
	}

	c, ok := s.complaints[complainer]
	// if the complaint is new, add it
	if !ok {
		s.complaints[complainer] = &complaint{
			received:       false,
			answerReceived: true,
		}
		// check the answer format
		if complainer == s.leaderIndex || len(data) != complainAnswerSize {
			s.complaints[complainer].validComplaint = true
			return invalid, nil
		}
		// read the complainer private share
		C.bn_read_bin((*C.bn_st)(&c.answer),
			(*C.uchar)(&data[1]),
			PrKeyLengthBLS_BLS12381,
		)
		return valid, nil
	}
	// complaint is not new in the map
	// check if the answer has been already received
	if c.answerReceived {
		return invalid, nil
	}
	c.answerReceived = true
	if complainer == s.leaderIndex || len(data) != complainAnswerSize {
		s.complaints[complainer].validComplaint = true
		return invalid, nil
	}

	// first flag check is a sanity check
	if c.received {
		// read the complainer private share
		C.bn_read_bin((*C.bn_st)(&c.answer),
			(*C.uchar)(&data[1]),
			PrKeyLengthBLS_BLS12381,
		)
		s.checkComplaint(complainer, c)
		return valid, nil
	}
	return invalid, nil
}
