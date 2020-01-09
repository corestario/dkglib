package msgs

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/corestario/dkglib/lib/alias"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type MsgSendDKGData struct {
	Data  *alias.DKGData
	Owner sdk.AccAddress
}

type RandDKGData struct {
	Data  *alias.DKGData `json:"data"`
	Owner sdk.AccAddress `json:"owner"`
}

func (m RandDKGData) String() string {
	return fmt.Sprintf("Data: %+v, Owner: %s", m.Data, m.Owner.String())
}

func NewMsgSendDKGData(data *alias.DKGData, owner sdk.AccAddress) MsgSendDKGData {
	return MsgSendDKGData{
		Data:  data,
		Owner: owner,
	}
}

// Route should return the name of the module
func (msg MsgSendDKGData) Route() string { return "randapp" }

// Type should return the action
func (msg MsgSendDKGData) Type() string { return "send_dkg_data" }

// ValidateBasic runs stateless checks on the message
func (msg MsgSendDKGData) ValidateBasic() error {
	if msg.Owner.Empty() {
		return errors.New("empty owner")
	}
	if err := msg.Data.ValidateBasic(); err != nil {
		return fmt.Errorf("data validation failed: %v", err)
	}
	return nil
}

// GetSignBytes encodes the message for signing.
func (msg MsgSendDKGData) GetSignBytes() []byte {
	b, err := json.Marshal(msg)
	if err != nil {
		panic(err)
	}
	return sdk.MustSortJSON(b)
}

// GetSigners defines whose signature is required
func (msg MsgSendDKGData) GetSigners() []sdk.AccAddress {
	return []sdk.AccAddress{msg.Owner}
}
