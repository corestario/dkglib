package lib

import "github.com/cosmos/cosmos-sdk/client/context"

type OnChainDKG struct {
	cli *context.CLIContext
}

func NewOnChainDKG(cli *context.CLIContext) *OnChainDKG {
	return &OnChainDKG{
		cli: cli,
	}
}

func (m *OnChainDKG) StartDKGRound() {

}
