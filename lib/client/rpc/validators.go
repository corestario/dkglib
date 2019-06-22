package rpc

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	tmtypes "github.com/tendermint/tendermint/types"

	"dgamingfoundation/dkglib/lib/client/context"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/rest"
)

// Validator output in bech32 format
type ValidatorOutput struct {
	Address          sdk.ConsAddress `json:"address"`
	PubKey           string          `json:"pub_key"`
	ProposerPriority int64           `json:"proposer_priority"`
	VotingPower      int64           `json:"voting_power"`
}

// Validators at a certain height output in bech32 format
type ResultValidatorsOutput struct {
	BlockHeight int64             `json:"block_height"`
	Validators  []ValidatorOutput `json:"validators"`
}

func (rvo ResultValidatorsOutput) String() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("block height: %d\n", rvo.BlockHeight))

	for _, val := range rvo.Validators {
		b.WriteString(
			fmt.Sprintf(`
  Address:          %s
  Pubkey:           %s
  ProposerPriority: %d
  VotingPower:      %d
		`,
				val.Address, val.PubKey, val.ProposerPriority, val.VotingPower,
			),
		)
	}

	return b.String()
}

func bech32ValidatorOutput(validator *tmtypes.Validator) (ValidatorOutput, error) {
	bechValPubkey, err := sdk.Bech32ifyConsPub(validator.PubKey)
	if err != nil {
		return ValidatorOutput{}, err
	}

	return ValidatorOutput{
		Address:          sdk.ConsAddress(validator.Address),
		PubKey:           bechValPubkey,
		ProposerPriority: validator.ProposerPriority,
		VotingPower:      validator.VotingPower,
	}, nil
}

func GetValidators(cliCtx context.CLIContext, height *int64) (ResultValidatorsOutput, error) {
	// get the node
	node, err := cliCtx.GetNode()
	if err != nil {
		return ResultValidatorsOutput{}, err
	}

	validatorsRes, err := node.Validators(height)
	if err != nil {
		return ResultValidatorsOutput{}, err
	}

	if !cliCtx.TrustNode {
		check, err := cliCtx.Verify(validatorsRes.BlockHeight)
		if err != nil {
			return ResultValidatorsOutput{}, err
		}

		if !bytes.Equal(check.ValidatorsHash, tmtypes.NewValidatorSet(validatorsRes.Validators).Hash()) {
			return ResultValidatorsOutput{}, fmt.Errorf("received invalid validatorset")
		}
	}

	outputValidatorsRes := ResultValidatorsOutput{
		BlockHeight: validatorsRes.BlockHeight,
		Validators:  make([]ValidatorOutput, len(validatorsRes.Validators)),
	}

	for i := 0; i < len(validatorsRes.Validators); i++ {
		outputValidatorsRes.Validators[i], err = bech32ValidatorOutput(validatorsRes.Validators[i])
		if err != nil {
			return ResultValidatorsOutput{}, err
		}
	}

	return outputValidatorsRes, nil
}

// REST

// Validator Set at a height REST handler
func ValidatorSetRequestHandlerFn(cliCtx context.CLIContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		height, err := strconv.ParseInt(vars["height"], 10, 64)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusBadRequest, "ERROR: Couldn't parse block height. Assumed format is '/validatorsets/{height}'.")
			return
		}

		chainHeight, err := GetChainHeight(cliCtx)
		if height > chainHeight {
			rest.WriteErrorResponse(w, http.StatusNotFound, "ERROR: Requested block height is bigger then the chain length.")
			return
		}

		output, err := GetValidators(cliCtx, &height)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		rest.PostProcessResponse(w, cdc, output, cliCtx.Indent)
	}
}

// Latest Validator Set REST handler
func LatestValidatorSetRequestHandlerFn(cliCtx context.CLIContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		output, err := GetValidators(cliCtx, nil)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		rest.PostProcessResponse(w, cdc, output, cliCtx.Indent)
	}
}
