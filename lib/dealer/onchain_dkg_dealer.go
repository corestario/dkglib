package dealer

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"

	"github.com/corestario/dkglib/lib/alias"
	tmtypes "github.com/tendermint/tendermint/alias"
	"github.com/tendermint/tendermint/libs/events"
	"github.com/tendermint/tendermint/libs/log"
	dkg "go.dedis.ch/kyber/v3/share/dkg/pedersen"
	vss "go.dedis.ch/kyber/v3/share/vss/pedersen"
)

type onChainDealer struct {
	*DKGDealer
	instance *dkg.DistKeyGenerator
	deals    map[string]*dkg.Deal
}

func (d *onChainDealer) GenerateTransitions() {
	d.transitions = []transition{
		d.SendCommits,
		d.SendDeals,
		d.ProcessDeals,
		d.ProcessCommits,
		d.ProcessResponses,
	}
}

func NewOnChainDKGDealer(
	validators *tmtypes.ValidatorSet,
	pv tmtypes.PrivValidator,
	sendMsgCb func(*alias.DKGData) error,
	eventFirer events.Fireable,
	logger log.Logger,
	startRound int,
) Dealer {
	return &onChainDealer{
		DKGDealer: NewDKGDealer(validators, pv, sendMsgCb, eventFirer, logger, startRound).(*DKGDealer),
	}
}

func (d *onChainDealer) SendCommits() (error, bool) {
	if !d.IsReady() {
		d.logger.Debug("DKG send deals: dealer is not ready")
		return nil, false
	}

	// TODO: fire event.

	for _, commit := range d.instance.GetDealer().Commits() {
		var (
			buf = bytes.NewBuffer(nil)
			enc = gob.NewEncoder(buf)
		)
		if err := enc.Encode(commit); err != nil {
			return fmt.Errorf("failed to encode public key: %v", err), false
		}
		err := d.SendMsgCb(&alias.DKGData{
			Type:    alias.DKGCommits,
			RoundID: d.roundID,
			Addr:    d.addrBytes,
			Data:    buf.Bytes(),
		})
		if err != nil {
			return fmt.Errorf("failed to send commit: %v", err), false
		}

	}

	return nil, true
}

func (d *onChainDealer) ProcessDeals() (error, bool) {
	if !d.IsDealsReady() {
		d.logger.Debug("DKGDealer process deals, deals are not ready")
		return nil, false
	}

	for dealerID, deal := range d.deals {
		//party does not have to verify their own deal
		if dealerID == d.participantID {
			continue
		}
		resp, err := d.instance.ProcessDeal(deal)
		if err != nil {
			return err
		}

		//commits verification
		allVerifiers := d.instance.Verifiers()
		verifier := allVerifiers[deal.Index]
		commitsOK, _ := ProcessDealCommits(verifier, deal, arcade)

		// if something goes wrong, party complains.
		if !resp.Response.Status || !commitsOK {
			config := d.instance.GetConfig()
			longterm := config.Longterm
			if err := arcade.DealComplaint(d.participantID, deal, longterm); err != nil {
				log.Println("failed to complain:", err)
			}
		}

		var (
			buf = bytes.NewBuffer(nil)
			enc = gob.NewEncoder(buf)
		)
		if err := enc.Encode(resp); err != nil {
			return fmt.Errorf("failed to encode public key: %v", err), false
		}
		err = d.SendMsgCb(&alias.DKGData{
			Type:    alias.DKGResponse,
			RoundID: d.roundID,
			Addr:    d.addrBytes,
			Data:    buf.Bytes(),
		})
		if err != nil {
			return fmt.Errorf("failed to send response: %v", err), false
		}

	}

	d.logger.Info("dkgState: processing deals")
	responseMessages, err := d.GetResponses()
	if err != nil {
		return fmt.Errorf("failed to get responses: %v", err), true
	}

	for _, responseMsg := range responseMessages {
		if err = d.SendMsgCb(responseMsg); err != nil {
			return fmt.Errorf("failed to sign message: %v", err), true
		}
	}

	d.logger.Debug("DKG process deals success")
	return err, true
}

func ProcessDealCommits(verifier *vss.Verifier, deal *dkg.Deal, arcade *Blockchain) (bool, error) {
	//verifier decryptDeal
	decryptedDeal, err := verifier.DecryptDeal(deal.Deal)
	if err != nil {
		return false, err
	}

	originalCommits, err := arcade.GetCommitsByID(int(deal.Index))
	if err != nil {
		return false, err
	}

	//check that commits on the chain and commits in the deal are met

	if len(originalCommits) != len(decryptedDeal.Commitments) {
		return false, errors.New("number of original commitmetns and number of commitments in the deal are not met")
	}

	for i := range originalCommits {
		if !originalCommits[i].Equal(decryptedDeal.Commitments[i]) {
			return false, errors.New("commits are different")
		}
	}

	return true, nil
}
