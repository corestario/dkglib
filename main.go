package main

import (
	"dgamingfoundation/dkglib/lib"
	"dgamingfoundation/dkglib/lib/client/keys"
	"dgamingfoundation/dkglib/lib/client/utils"
	"fmt"
	"os"
	"os/user"
	"path"
	"strconv"
	"sync"
	"time"

	cliCTX "dgamingfoundation/dkglib/lib/client/context"
	authtxb "dgamingfoundation/dkglib/lib/client/txbuilder"

	sdk "github.com/cosmos/cosmos-sdk/types"
	app "github.com/dgamingfoundation/randapp"
	"github.com/spf13/viper"
	"github.com/tendermint/tendermint/libs/events"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

const (
	nodeEndpoint  = "tcp://localhost:26657" // TODO: get this from command line args
	chainID       = "rchain"
	vfrHome       = ""
	height        = 0
	trustNode     = false
	broadcastMode = "sync"
	genOnly       = false
	validatorName = "validator"
)

var cliHome = "~/.rcli" // TODO: get this from command line args

func init() {
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}

	cliHome = usr.HomeDir + "/" + ".rcli"
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

	var MPV types.PrivValidator
	var OC *lib.OnChainDKG
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
			oc := lib.NewOnChainDKG(cli, txBldr)
			if err := oc.StartRound(types.NewValidatorSet(vals), pval, mockF, logger, 0); err != nil {
				panic(fmt.Sprintf("failed to start round: %v", err))
			}
			OC = oc
			MPV = pval
			tk := time.NewTicker(time.Millisecond * 2000)
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
	tick := time.NewTicker(time.Second * 20)
	for {
		func() {
			<-tick.C
			mu.Lock()
			defer mu.Unlock()
			for k, v := range MP {
				go v.StartRound(types.NewValidatorSet(vals), k, mockF, logger, 0)
			}
			//if err := OC.StartRound(types.NewValidatorSet(vals), MPV, mockF, logger, 0); err != nil {
			//	panic(fmt.Sprintf("failed to start round: %v", err))
			//}
		}()
	}

	wg.Wait()
	fmt.Println("All instances finished DKG, O.K.")
}

func getValidatorEnv() (*types.Validator, types.PrivValidator) {
	pv := types.NewMockPV()
	return types.NewValidator(pv.GetPubKey(), 1), pv
}

func getTools(vName string) (*cliCTX.CLIContext, *authtxb.TxBuilder, error) {
	//if err := initConfig(validatorName); err != nil {
	//	return nil, nil, fmt.Errorf("could not read config: %v", err)
	//}
	cdc := app.MakeCodec()
	cliCtx, err := cliCTX.NewCLIContext(chainID, nodeEndpoint, validatorName+vName, genOnly, broadcastMode, vfrHome, height, trustNode, cliHome+vName)
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
	txBldr := authtxb.NewTxBuilder(utils.GetTxEncoder(cdc), accNumber, 0, 40000000, 0.0, false, cliCtx.Verifier.ChainID(), "", nil, nil).WithKeybase(kb)
	if err := cliCtx.EnsureAccountExists(); err != nil {
		return nil, nil, fmt.Errorf("failed to find account: %v", err)
	}

	return &cliCtx, &txBldr, nil
}

func initConfig(_ string) error {
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
