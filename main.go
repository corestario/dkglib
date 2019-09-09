package main

import (
	"fmt"
	"os"
	"os/user"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/bank"
	"github.com/cosmos/cosmos-sdk/x/staking"
	authtxb "github.com/dgamingfoundation/cosmos-utils/client/authtypes"
	"github.com/dgamingfoundation/cosmos-utils/client/context"
	"github.com/dgamingfoundation/cosmos-utils/client/utils"
	"github.com/dgamingfoundation/dkglib/lib"
	dkgtypes "github.com/dgamingfoundation/dkglib/lib/types"
	"github.com/tendermint/tendermint/libs/events"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

const (
	nodeEndpoint  = "tcp://localhost:26657" // TODO: get this from command line args
	chainID       = "rchain"
	validatorName = "validator"
	passphrase    = "12345678"
)

var cliHome = "~/.rcli" // TODO: get this from command line args

func init() {
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}

	cliHome = path.Join(usr.HomeDir, ".rcli")
}

func MakeCodec() *codec.Codec {
	var cdc = codec.New()
	auth.RegisterCodec(cdc)
	bank.RegisterCodec(cdc)
	cdc.RegisterConcrete(dkgtypes.MsgSendDKGData{}, "randapp/SendDKGData", nil)
	staking.RegisterCodec(cdc)
	sdk.RegisterCodec(cdc)
	codec.RegisterCrypto(cdc)
	return cdc
}

func main() {
	var (
		mockF  = &MockFirer{}
		logger = log.NewTMLogger(os.Stdout)
		vals   []*types.Validator
		pvals  []types.PrivValidator
	)
	for i := 0; i < 4; i++ {
		v, pv := getValidatorEnv()
		vals, pvals = append(vals, v), append(pvals, pv)

	}

	var mu sync.Mutex
	MP := make(map[types.PrivValidator]lib.OnChainDKG)
	wg := &sync.WaitGroup{}
	for k, pval := range pvals {
		cli, txBldr, err := getTools(strconv.Itoa(k))
		if err != nil {
			fmt.Printf("failed to get a randapp client: %v", err)
			os.Exit(1)
		}

		wg.Add(1)

		oc := lib.NewOnChainDKG(cli, txBldr)
		pv := pval
		mu.Lock()
		MP[pv] = *oc
		mu.Unlock()
		go func(pval types.PrivValidator) {
			if err := oc.StartRound(types.NewValidatorSet(vals), pval, mockF, logger, 0); err != nil {
				panic(fmt.Sprintf("failed to start round: %v", err))
			}
			tk := time.NewTicker(time.Millisecond * 3000)
			for {
				select {
				case <-tk.C:
					if err, ok := oc.ProcessBlock(); err != nil {
						panic(fmt.Sprintf("failed to start round: %v", err))
					} else if ok {
						wg.Done()
						return
					}
				}
			}
		}(pval)
	}

	wg.Wait()
	fmt.Println("All instances finished DKG, O.K.")
}

func getValidatorEnv() (*types.Validator, types.PrivValidator) {
	pv := types.NewMockPV()
	return types.NewValidator(pv.GetPubKey(), 1), pv
}

func getTools(vName string) (*context.Context, *authtxb.TxBuilder, error) {
	cdc := MakeCodec()
	ctx, err := context.NewContext(chainID, nodeEndpoint, cliHome+vName)
	if err != nil {
		return nil, nil, err
	}
	ctx = ctx.WithCodec(cdc)
	addr, _, err := context.GetFromFields(validatorName+vName, cliHome+vName)
	if err != nil {
		return nil, nil, err
	}
	ctx = ctx.WithFromName(validatorName + vName).WithPassphrase(passphrase).WithFromAddress(addr).WithFrom(validatorName + vName)

	accRetriever := auth.NewAccountRetriever(ctx)
	accNumber, accSequence, err := accRetriever.GetAccountNumberSequence(addr)
	if err != nil {
		return nil, nil, err
	}
	kb, err := keys.NewKeyBaseFromDir(ctx.Home)
	if err != nil {
		return nil, nil, err
	}
	txBldr := authtxb.NewTxBuilder(utils.GetTxEncoder(cdc), accNumber, accSequence, 400000, 0.0, false, ctx.Verifier.ChainID(), "", nil, nil).WithKeybase(kb)
	if err := ctx.EnsureAccountExists(); err != nil {
		return nil, nil, fmt.Errorf("failed to find account: %v", err)
	}

	return &ctx, &txBldr, nil
}

type MockFirer struct{}

func (m *MockFirer) FireEvent(event string, data events.EventData) {}
