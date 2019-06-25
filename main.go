package main

import (
	"dgamingfoundation/dkglib/lib"
	"dgamingfoundation/dkglib/lib/client/keys"
	"dgamingfoundation/dkglib/lib/client/utils"
	"fmt"
	"os"
	"path"
	"sync"
	"time"

	cliCTX "dgamingfoundation/dkglib/lib/client/context"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtxb "github.com/cosmos/cosmos-sdk/x/auth/client/txbuilder"
	"github.com/dgamingfoundation/randapp/util"
	"github.com/spf13/viper"
	"github.com/tendermint/tendermint/libs/events"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

const (
	cliHome      = "/Users/pr0n00gler/.rcli" // TODO: get this from command line args
	nodeEndpoint = "tcp://localhost:26657"   // TODO: get this from command line args
)

func main() {
	var (
		mockF  = &MockFirer{}
		logger = log.NewTMLogger(os.Stdout)
		vals   []*types.Validator
		pvals  []types.PrivValidator
	)
	for i := 0; i < 1; i++ {
		v, pv := getValidatorEnv()
		vals, pvals = append(vals, v), append(pvals, pv)

	}

	wg := &sync.WaitGroup{}
	for _, pval := range pvals {
		cli, txBldr, err := getTools("validator1")
		if err != nil {
			fmt.Printf("failed to get a randapp client: %v", err)
			os.Exit(1)
		}

		wg.Add(1)

		go func(pval types.PrivValidator) {
			oc := lib.NewOnChainDKG(cli, txBldr)
			if err := oc.StartRound(types.NewValidatorSet(vals), pval, mockF, logger, 0); err != nil {
				panic(fmt.Sprintf("failed to start round: %v", err))
			}

			tk := time.NewTicker(time.Second)
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

func getTools(validatorName string) (*cliCTX.CLIContext, *authtxb.TxBuilder, error) {
	if err := initConfig(validatorName); err != nil {
		return nil, nil, fmt.Errorf("could not read config: %v", err)
	}
	cdc := util.MakeCodec()
	cliCtx, err := cliCTX.NewCLIContext("NFTChain", "localhost:26657", validatorName, false, "", "", 0, false, false, "sync", false, false, false, false, cliHome)
	if err != nil {
		return nil, nil, err
	}
	cliCtx = cliCtx.WithCodec(cdc).WithAccountDecoder(cdc)
	accNumber, err := cliCtx.GetAccountNumber(cliCtx.FromAddress)
	if err != nil {
		return nil, nil, err
	}
	kb, err := keys.NewKeyBaseFromDir(cliCtx.Home)
	if err != nil {
		return nil, nil, err
	}
	txBldr := authtxb.NewTxBuilder(utils.GetTxEncoder(cdc), accNumber, 0, 0, 0.0, false, cliCtx.Verifier.ChainID(), "", nil, nil).WithKeybase(kb)
	if err := cliCtx.EnsureAccountExists(); err != nil {
		return nil, nil, fmt.Errorf("failed to find account: %v", err)
	}

	return &cliCtx, &txBldr, nil
}

func initConfig(validatorName string) error {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount(sdk.Bech32PrefixAccAddr, sdk.Bech32PrefixAccPub)
	config.SetBech32PrefixForValidator(sdk.Bech32PrefixValAddr, sdk.Bech32PrefixValPub)
	config.SetBech32PrefixForConsensusNode(sdk.Bech32PrefixConsAddr, sdk.Bech32PrefixConsPub)
	config.Seal()

	cfgFile := path.Join(cliHome, "config", "config.toml")
	if _, err := os.Stat(cfgFile); err == nil {
		viper.SetConfigFile(cfgFile)

		if err := viper.ReadInConfig(); err != nil {
			return err
		}
	}

	return nil
}

type MockFirer struct{}

func (m *MockFirer) FireEvent(event string, data events.EventData) {}
