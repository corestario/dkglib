package main

import (
	"flag"
	"fmt"
	"os"
	"os/user"
	"path"
	"strconv"
	"time"

	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authTypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	authtxb "github.com/dgamingfoundation/cosmos-utils/client/authtypes"
	"github.com/dgamingfoundation/cosmos-utils/client/context"
	"github.com/dgamingfoundation/cosmos-utils/client/utils"
	msgs "github.com/dgamingfoundation/dkglib/lib/msgs"
	onChain "github.com/dgamingfoundation/dkglib/lib/onChain"
	types "github.com/tendermint/tendermint/alias"
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
