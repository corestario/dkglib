package dealer

import (
	"bytes"
	"encoding/gob"
	"fmt"

	"github.com/corestario/dkglib/lib/alias"
	"github.com/tendermint/tendermint/libs/events"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
	dkg "go.dedis.ch/kyber/v3/share/dkg/rabin"
)

type DKGMockDontSendOneCommit struct {
	Dealer
}

func NewDKGMockDealerNoCommit(validators *types.ValidatorSet, pv types.PrivValidator, sendMsgCb func([]*alias.DKGData) error, eventFirer events.Fireable, logger log.Logger, startRound int) Dealer {
	return &DKGMockDontSendOneCommit{NewDKGDealer(validators, pv, sendMsgCb, eventFirer, logger, startRound)}
}

func (m *DKGMockDontSendOneCommit) Start() error {
	err := m.Dealer.Start()
	if err != nil {
		return err
	}
	m.GenerateTransitions()
	return nil
}

func (m *DKGMockDontSendOneCommit) GenerateTransitions() {
	m.Dealer.SetTransitions([]transition{
		// Phase I
		m.Dealer.SendDeals,
		m.Dealer.ProcessDeals,
		m.Dealer.ProcessResponses,
		m.ProcessJustifications,
		// Phase II
		m.Dealer.ProcessCommits,
		m.Dealer.ProcessComplaints,
		m.Dealer.ProcessReconstructCommits,
	})
}

func (m *DKGMockDontSendOneCommit) ProcessJustifications() (err error, ready bool) {
	if !m.IsJustificationsReady() {
		return nil, false
	}

	commits, err := m.GetCommits()
	if err != nil {
		return err, true
	}

	var (
		buf = bytes.NewBuffer(nil)
		enc = gob.NewEncoder(buf)
	)
	if err := enc.Encode(commits); err != nil {
		return fmt.Errorf("failed to encode response: %v", err), true
	}

	state := m.Dealer.GetState()

	message := &alias.DKGData{
		Type:        alias.DKGCommits,
		RoundID:     state.roundID,
		Addr:        state.addrBytes,
		Data:        buf.Bytes(),
		NumEntities: len(commits.Commitments),
	}

	return m.SendMsgCb([]*alias.DKGData{message}), true
}

func (m *DKGMockDontSendOneCommit) GetCommits() (*dkg.SecretCommits, error) {
	commits, err := m.Dealer.GetCommits()

	// remove one response message
	commits.Commitments[len(commits.Commitments)-1] = nil
	commits.Commitments = commits.Commitments[:len(commits.Commitments)-1]

	return commits, err
}

type DKGMockDontSendAnyCommits struct {
	Dealer
	logger log.Logger
}

func NewDKGMockDealerAnyCommits(validators *types.ValidatorSet, pv types.PrivValidator, sendMsgCb func([]*alias.DKGData) error, eventFirer events.Fireable, logger log.Logger, startRound int) Dealer {
	return &DKGMockDontSendAnyCommits{NewDKGDealer(validators, pv, sendMsgCb, eventFirer, logger, startRound), logger}
}

func (m *DKGMockDontSendAnyCommits) Start() error {
	err := m.Dealer.Start()
	if err != nil {
		return err
	}
	m.GenerateTransitions()
	return nil
}

func (m *DKGMockDontSendAnyCommits) GenerateTransitions() {
	m.Dealer.SetTransitions([]transition{
		// Phase I
		m.Dealer.SendDeals,
		m.Dealer.ProcessDeals,
		m.Dealer.ProcessResponses,
		m.ProcessJustifications,
		// Phase II
		m.Dealer.ProcessCommits,
		m.Dealer.ProcessComplaints,
		m.Dealer.ProcessReconstructCommits,
	})
}

func (m *DKGMockDontSendAnyCommits) ProcessJustifications() (error, bool) {
	if !m.Dealer.IsJustificationsReady() {
		return nil, false
	}

	m.logger.Info("dkgState: sending commits", "commits", 0)

	return nil, true
}
