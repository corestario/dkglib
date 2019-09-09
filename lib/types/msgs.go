package types

import (
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type MsgSendDKGData struct {
	Data  *DKGData
	Owner sdk.AccAddress
}

type RandDKGData struct {
	Data  *DKGData       `json:"data"`
	Owner sdk.AccAddress `json:"owner"`
}

func (m RandDKGData) String() string {
	return fmt.Sprintf("Data: %+v, Owner: %s", m.Data, m.Owner.String())
}

func NewMsgSendDKGData(data *DKGData, owner sdk.AccAddress) MsgSendDKGData {
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
func (msg MsgSendDKGData) ValidateBasic() sdk.Error {
	if msg.Owner.Empty() {
		return sdk.ErrInvalidAddress(msg.Owner.String())
	}
	if err := msg.Data.ValidateBasic(); err != nil {
		return sdk.ErrUnknownRequest(fmt.Sprintf("data validation failed: %v", err))
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
