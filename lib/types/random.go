package types

import (
	"github.com/corestario/dkglib/lib/blsShare"
)

//DKG events
const (
	EventDKGData                        = "DKGData"
	EventDKGStart                       = "DKGStart"
	EventDKGPubKeyReceived              = "DKGPubKeyReceived"
	EventDKGDealsProcessed              = "DKGDealsProcessed"
	EventDKGResponsesProcessed          = "DKGResponsesProcessed"
	EventDKGJustificationsProcessed     = "DKGJustificationsProcessed"
	EventDKGInstanceCertified           = "DKGInstanceCertified"
	EventDKGCommitsProcessed            = "DKGCommitsProcessed"
	EventDKGComplaintProcessed          = "DKGComplaintProcessed"
	EventDKGReconstructCommitsProcessed = "DKGReconstructCommitsProcessed"
	EventDKGSuccessful                  = "DKGSuccessful"
	EventDKGKeyChange                   = "DKGKeyChange"
)

type Verifier interface {
	Sign(data []byte) ([]byte, error)
	VerifyRandomShare(addr string, prevRandomData, currRandomData []byte) error
	VerifyRandomData(prevRandomData, currRandomData []byte) error
	Recover(msg []byte, precommits []blsShare.BLSSigner) ([]byte, error)
	IsNil() bool
}

type MockVerifier struct{}

func (m *MockVerifier) Sign(data []byte) ([]byte, error) {
	return []byte{0}, nil
}
func (m *MockVerifier) VerifyRandomShare(addr string, prevRandomData, currRandomData []byte) error {
	return nil
}
func (m *MockVerifier) VerifyRandomData(prevRandomData, currRandomData []byte) error {
	return nil
}
func (m *MockVerifier) Recover(msg []byte, precommits []blsShare.BLSSigner) ([]byte, error) {
	return []byte{}, nil
}
func (m *MockVerifier) IsNil() bool {
	return false
}
