package main

import (
	"fmt"
	"os"
	"path"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/context"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/dgamingfoundation/randapp/util"
	"github.com/dgamingfoundation/randapp/x/randapp"
	"github.com/spf13/viper"
)

const (
	queryRoute   = "randapp"
	cliHome      = "/Users/andrei/.rcli"   // TODO: get this from command line args
	nodeEndpoint = "tcp://localhost:26657" // TODO: get this from command line args
)

func main() {
	cli, err := getRandappClient()
	if err != nil {
		fmt.Printf("failed to get a randapp client: %v", err)
		os.Exit(1)
	}

	res, err := cliCtx.QueryWithData(fmt.Sprintf("custom/%s/item/%s", queryRoute, "42"), nil)
	if err != nil {
		fmt.Printf("could not get item - %s: %v \n", id, err)
		os.Exit(1)
	}

	var out randapp.Item
	cdc.MustUnmarshalJSON(res, &out)
	if err := cliCtx.PrintOutput(out); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func getRandappClient() (*context.CLIContext, error) {
	if err := initConfig(); err != nil {
		return nil, fmt.Errorf("could not read config: %v", err)
	}
	cliCtx := context.NewCLIContext().WithCodec(util.MakeCodec())
	return &cliCtx, nil
}

func initConfig() error {
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
	viper.Set(client.FlagNode, nodeEndpoint)

	return nil
}
