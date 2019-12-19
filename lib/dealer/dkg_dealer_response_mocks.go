package dealer

import (
	"errors"

	"github.com/corestario/dkglib/lib/alias"
	"github.com/tendermint/tendermint/libs/events"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

type DKGMockDontSendOneResponse struct {
	Dealer
	logger log.Logger
}

func NewDKGMockDealerNoResponse(validators *types.ValidatorSet, pv types.PrivValidator, sendMsgCb func(*alias.DKGData) error, eventFirer events.Fireable, logger log.Logger, startRound int) Dealer {
	return &DKGMockDontSendOneResponse{NewDKGDealer(validators, pv, sendMsgCb, eventFirer, logger, startRound), logger}
}

func (m *DKGMockDontSendOneResponse) Start() error {
	err := m.Dealer.Start()
	if err != nil {
		return err
	}
	m.GenerateTransitions()
	return nil
}

func (m *DKGMockDontSendOneResponse) GenerateTransitions() {
	m.Dealer.SetTransitions([]transition{
		// Phase I
		m.Dealer.SendDeals,
		m.ProcessDeals,
		m.Dealer.ProcessResponses,
		m.Dealer.ProcessJustifications,
		// Phase II
		m.Dealer.ProcessCommits,
		m.Dealer.ProcessComplaints,
		m.Dealer.ProcessReconstructCommits,
	})
}

func (m *DKGMockDontSendOneResponse) ProcessDeals() (error, bool) {
	if !m.Dealer.IsDealsReady() {
		return nil, false
	}

	messages, err := m.GetDeals()
	if err != nil {
		return err, true
	}
	for _, msg := range messages {
		if err = m.Dealer.SendMsgCb(msg); err != nil {
			return err, true
		}
	}

	m.logger.Info("dkgState: sending responses", "responses", len(messages))

	return nil, true
}

func (m *DKGMockDontSendOneResponse) GetResponses() ([]*alias.DKGData, error) {
	responses, err := m.Dealer.GetResponses()
	if len(responses) == 0 {
		return nil, errors.New("DKGMockDontSendOneResponse got empty Responses")
	}

	// remove one response message
	responses[len(responses)-1] = nil
	responses = responses[:len(responses)-1]

	return responses, err
}

type DKGMockDontSendAnyResponses struct {
	Dealer
	logger log.Logger
}

func NewDKGMockDealerAnyResponses(validators *types.ValidatorSet, pv types.PrivValidator, sendMsgCb func(*alias.DKGData) error, eventFirer events.Fireable, logger log.Logger, startRound int) Dealer {
	return &DKGMockDontSendAnyResponses{NewDKGDealer(validators, pv, sendMsgCb, eventFirer, logger, startRound), logger}
}

func (m *DKGMockDontSendAnyResponses) Start() error {
	err := m.Dealer.Start()
	if err != nil {
		return err
	}
	m.GenerateTransitions()
	return nil
}

func (m *DKGMockDontSendAnyResponses) GenerateTransitions() {
	m.Dealer.SetTransitions([]transition{
		// Phase I
		m.Dealer.SendDeals,
		m.ProcessDeals,
		m.Dealer.ProcessResponses,
		m.Dealer.ProcessJustifications,
		// Phase II
		m.Dealer.ProcessCommits,
		m.Dealer.ProcessComplaints,
		m.Dealer.ProcessReconstructCommits,
	})
}

func (m *DKGMockDontSendAnyResponses) ProcessDeals() (error, bool) {
	if !m.Dealer.IsDealsReady() {
		return nil, false
	}

	m.logger.Info("dkgState: sending responses", "responses", 0)

	return nil, true
}
