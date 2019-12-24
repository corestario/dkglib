package main

import (
	"flag"
	"fmt"
	"os"
	"os/user"
	"path"
	"strconv"
	"time"

	authtxb "github.com/corestario/cosmos-utils/client/authtypes"
	"github.com/corestario/cosmos-utils/client/context"
	"github.com/corestario/cosmos-utils/client/utils"
	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authTypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	msgs "github.com/dgamingfoundation/dkglib/lib/msgs"
	onChain "github.com/dgamingfoundation/dkglib/lib/onChain"
	types "github.com/tendermint/tendermint/alias"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/libs/events"
	"github.com/tendermint/tendermint/libs/log"
)

const (
	nodeEndpoint  = "tcp://localhost:26657" // TODO: get this from command line args
	chainID       = "rchain"
	validatorName = "validator"
	passphrase    = "12345678"
)

var cliHome = "~/.rcli" // TODO: get this from command line args

func init() {
	populateMocks()
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}

	cliHome = path.Join(usr.HomeDir, ".rcli")
}

func MakeCodec() *codec.Codec {
	var cdc = codec.New()
	authTypes.RegisterCodec(cdc)
	cdc.RegisterConcrete(msgs.MsgSendDKGData{}, "randapp/SendDKGData", nil)
	sdk.RegisterCodec(cdc)
	codec.RegisterCrypto(cdc)
	return cdc
}

func main() {
	numPtr := flag.String("num", "0", "a string number")
	flag.Parse()

	var (
		mockF  = &MockFirer{}
		logger = log.NewTMLogger(os.Stdout)
	)

	numStr := "0"
	if numPtr != nil {
		numStr = *numPtr
	}

	num, err := strconv.Atoi(numStr)
	if err != nil {
		panic(err)
	}
	pval := MockPVs[num]

	cli, txBldr, err := getTools(numStr)
	if err != nil {
		fmt.Printf("failed to get a randapp client: %v", err)
		os.Exit(1)
	}

	oc := onChain.NewOnChainDKG(cli, txBldr)
	if err := oc.StartRound(types.NewValidatorSet(MockValidators), pval, mockF, logger, 0); err != nil {
		panic(fmt.Sprintf("failed to start round: %v", err))
	}
	tk := time.NewTicker(time.Millisecond * 3000)
	for {
		select {
		case <-tk.C:
			if err, ok := oc.ProcessBlock(); err != nil {
				panic(fmt.Sprintf("failed to start round: %v", err))
			} else if ok {
				fmt.Println("All instances finished DKG, O.K.")

				return
			}
		}
	}

}

func getTools(vName string) (*context.Context, *authtxb.TxBuilder, error) {
	cdc := MakeCodec()
	ctx, err := context.NewContextWithDelay(chainID, nodeEndpoint, cliHome+vName)
	if err != nil {
		return nil, nil, err
	}

	ctx.WithCodec(cdc)
	addr, _, err := context.GetFromFields(validatorName+vName, cliHome+vName)
	if err != nil {
		return nil, nil, err
	}
	ctx.WithFromName(validatorName + vName).WithPassphrase(passphrase).WithFromAddress(addr).WithFrom(validatorName + vName)

	accRetriever := authTypes.NewAccountRetriever(ctx)
	accNumber, accSequence, err := accRetriever.GetAccountNumberSequence(addr)
	if err != nil {
		return nil, nil, err
	}
	kb, err := keys.NewKeyBaseFromDir(ctx.Home)
	if err != nil {
		return nil, nil, err
	}

	for ctx.GetVerifier() == nil {
		time.Sleep(time.Second)
	}

	txBldr := authtxb.NewTxBuilder(utils.GetTxEncoder(cdc), accNumber, accSequence, 400000, 0.0, false, ctx.GetVerifier().ChainID(), "", nil, nil).WithKeybase(kb)
	if err := ctx.EnsureAccountExists(); err != nil {
		return nil, nil, fmt.Errorf("failed to find account: %v", err)
	}

	return ctx, &txBldr, nil
}

type MockFirer struct{}

func (m *MockFirer) FireEvent(event string, data events.EventData) {}

var MockValidators []*types.Validator
var MockPVs []*types.MockPV

func populateMocks() {
	MockValidators = make([]*types.Validator, 0)
	MockPVs = make([]*types.MockPV, 0)

	MockValidators = append(MockValidators, &types.Validator{Address: []byte("36B59C3FB528ECB5F6CBA9A80BC3DC292A52C0B1"),
		PubKey: ed25519.PubKeyEd25519{0xb8, 0x5d, 0xd0, 0x4c, 0x7a, 0x9f, 0xeb, 0x87, 0x3e, 0x74, 0x96, 0xaa,
			0x13, 0xa5, 0x30, 0x65, 0x9d, 0xe7, 0x5a, 0x94, 0xe, 0x82, 0x34, 0xd, 0xb7, 0xe5, 0x85, 0x57, 0x2b, 0x39, 0xce, 0xb0},
		VotingPower: 1, ProposerPriority: 0},
	)
	MockValidators = append(MockValidators, &types.Validator{Address: []byte("BD6D3BDA898316A16CB84D71EA39EF574BDE1B50"),
		PubKey: ed25519.PubKeyEd25519{0x79, 0x4c, 0x87, 0x5b, 0xd5, 0xd5, 0x55, 0x69, 0x57, 0xcb, 0xf1, 0xa9,
			0x22, 0x56, 0xec, 0xd0, 0x10, 0x51, 0xa, 0x77, 0xf7, 0x19, 0x9e, 0x4c, 0x9a, 0x41, 0x56, 0x4, 0xd3, 0x6c, 0x79, 0x95},
		VotingPower: 1, ProposerPriority: 0},
	)
	MockValidators = append(MockValidators, &types.Validator{Address: []byte("3275076B1ACCA2803847DE373B03F6AB2B8A31D1"),
		PubKey: ed25519.PubKeyEd25519{0xc1, 0x98, 0x2d, 0x42, 0x7f, 0x6a, 0xbc, 0x7a, 0x4c, 0x3b, 0xd0, 0x69,
			0xba, 0xd4, 0x4d, 0xa6, 0x8, 0xfd, 0xe4, 0x15, 0x2f, 0xcf, 0x61, 0xf5, 0xfe, 0x93, 0xfb, 0x83, 0x2b, 0xef, 0xcf, 0xbe},
		VotingPower: 1, ProposerPriority: 0},
	)
	MockValidators = append(MockValidators, &types.Validator{Address: []byte("B21F500A064E2CF82526A5B8D134AA273F023736"),
		PubKey: ed25519.PubKeyEd25519{0xf0, 0xcf, 0xda, 0x13, 0x8a, 0x7, 0xa1, 0xa6, 0xf8, 0xec, 0x90, 0x32,
			0x78, 0x3c, 0x2e, 0xda, 0xe, 0x9d, 0x9e, 0xc6, 0xbd, 0x2a, 0xfe, 0xaa, 0x34, 0x58, 0x73, 0xe7, 0x79, 0x73, 0xbc, 0x65},
		VotingPower: 1, ProposerPriority: 0},
	)

	MockPVs = append(MockPVs, types.NewMockPVWithParams(
		ed25519.PrivKeyEd25519{0x3c, 0xe4, 0xaa, 0x5a, 0xe7, 0x35, 0x78, 0xd, 0x89, 0x92, 0x21, 0x90, 0xfd, 0x8f,
			0x4a, 0xe2, 0xf6, 0x69, 0xad, 0x58, 0xc5, 0x26, 0xf4, 0x32, 0x95, 0xe5, 0x1, 0xac, 0x4b, 0xe8, 0x11,
			0xce, 0xb8, 0x5d, 0xd0, 0x4c, 0x7a, 0x9f, 0xeb, 0x87, 0x3e, 0x74, 0x96, 0xaa, 0x13, 0xa5, 0x30,
			0x65, 0x9d, 0xe7, 0x5a, 0x94, 0xe, 0x82, 0x34, 0xd, 0xb7, 0xe5, 0x85, 0x57, 0x2b, 0x39, 0xce, 0xb0},
		false, false,
	))
	MockPVs = append(MockPVs, types.NewMockPVWithParams(
		ed25519.PrivKeyEd25519{0x51, 0x57, 0x7b, 0x48, 0x58, 0x93, 0x1d, 0xb1, 0x7b, 0x44, 0x35, 0x5d, 0x7, 0xb,
			0x1f, 0xe, 0x2a, 0xa6, 0xa5, 0x70, 0x8, 0x49, 0x69, 0xdd, 0xe1, 0xef, 0x9d, 0x7f, 0x15, 0x51, 0xb6,
			0x97, 0x79, 0x4c, 0x87, 0x5b, 0xd5, 0xd5, 0x55, 0x69, 0x57, 0xcb, 0xf1, 0xa9, 0x22, 0x56, 0xec, 0xd0,
			0x10, 0x51, 0xa, 0x77, 0xf7, 0x19, 0x9e, 0x4c, 0x9a, 0x41, 0x56, 0x4, 0xd3, 0x6c, 0x79, 0x95},
		false, false,
	))
	MockPVs = append(MockPVs, types.NewMockPVWithParams(
		ed25519.PrivKeyEd25519{0x89, 0x63, 0xc7, 0x6d, 0xd9, 0xd6, 0x92, 0x4d, 0x5e, 0xfe, 0x34, 0xa, 0x31, 0xac,
			0xee, 0xe6, 0x32, 0xf6, 0xe2, 0xa9, 0x26, 0x5d, 0x47, 0x40, 0x3c, 0xfc, 0xef, 0xb5, 0xf1, 0x98, 0x6a,
			0xd9, 0xc1, 0x98, 0x2d, 0x42, 0x7f, 0x6a, 0xbc, 0x7a, 0x4c, 0x3b, 0xd0, 0x69, 0xba, 0xd4, 0x4d, 0xa6,
			0x8, 0xfd, 0xe4, 0x15, 0x2f, 0xcf, 0x61, 0xf5, 0xfe, 0x93, 0xfb, 0x83, 0x2b, 0xef, 0xcf, 0xbe},
		false, false,
	))
	MockPVs = append(MockPVs, types.NewMockPVWithParams(
		ed25519.PrivKeyEd25519{0x1, 0x3d, 0x2f, 0x24, 0xf0, 0x37, 0x40, 0x9f, 0x72, 0x2b, 0x52, 0x48, 0x35, 0xb3,
			0xcb, 0xd6, 0xf4, 0x27, 0x52, 0x49, 0xef, 0xab, 0x60, 0xf6, 0x14, 0x2e, 0xca, 0x62, 0x48, 0xe8, 0x9b,
			0x4d, 0xf0, 0xcf, 0xda, 0x13, 0x8a, 0x7, 0xa1, 0xa6, 0xf8, 0xec, 0x90, 0x32, 0x78, 0x3c, 0x2e, 0xda,
			0xe, 0x9d, 0x9e, 0xc6, 0xbd, 0x2a, 0xfe, 0xaa, 0x34, 0x58, 0x73, 0xe7, 0x79, 0x73, 0xbc, 0x65},
		false, false,
	))

}
