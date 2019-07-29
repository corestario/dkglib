package context

import (
	"fmt"

	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/cosmos/cosmos-sdk/codec"
	cryptokeys "github.com/cosmos/cosmos-sdk/crypto/keys"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/dgamingfoundation/dkglib/lib/client/keys"

	"github.com/tendermint/tendermint/libs/log"
	tmlite "github.com/tendermint/tendermint/lite"
	tmliteProxy "github.com/tendermint/tendermint/lite/proxy"
	rpcclient "github.com/tendermint/tendermint/rpc/client"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	verifier     tmlite.Verifier
	verifierHome string
)

// CLIContext implements a typical CLI context created in SDK modules for
// transaction handling and queries.
type CLIContext struct {
	Codec         *codec.Codec
	AccDecoder    auth.AccountDecoder
	Client        rpcclient.Client
	Keybase       cryptokeys.Keybase
	Output        io.Writer
	OutputFormat  string
	Height        int64
	NodeURI       string
	From          string
	AccountStore  string
	TrustNode     bool
	UseLedger     bool
	BroadcastMode string
	PrintResponse bool
	Verifier      tmlite.Verifier
	VerifierHome  string
	Simulate      bool
	GenerateOnly  bool
	FromAddress   sdk.AccAddress
	FromName      string
	Indent        bool
	SkipConfirm   bool
	Home          string
	Passphrase    string
}

// NewCLIContext returns a new initialized CLIContext
func NewCLIContext(chainID string, nodeURI string, from string, genOnly bool, broadcastMode string, vfrHome string, height int64, trustNode bool, home string, passphrase string) (CLIContext, error) {
	var (
		rpc rpcclient.Client
		cli CLIContext
		err error
	)

	if nodeURI != "" {
		rpc = rpcclient.NewHTTP(nodeURI, "/websocket")
	}
	fromAddress, fromName, err := GetFromFields(from, genOnly, home)
	if err != nil {
		return cli, err
	}

	// We need to use a single verifier for all contexts
	if verifier == nil || verifierHome != vfrHome {
		if verifier, err = createVerifier(chainID, home, nodeURI, trustNode); err != nil {
			return cli, err
		}
		verifierHome = vfrHome
	}

	cli = CLIContext{
		Client:        rpc,
		Output:        os.Stdout,
		NodeURI:       nodeURI,
		AccountStore:  auth.StoreKey,
		From:          from,
		Height:        height,
		TrustNode:     trustNode,
		Verifier:      verifier,
		GenerateOnly:  genOnly,
		FromAddress:   fromAddress,
		FromName:      fromName,
		Home:          home,
		BroadcastMode: broadcastMode,
		Passphrase:    passphrase,
	}
	return cli, nil
}

func createVerifier(chainID string, home string, nodeURI string, trustNode bool) (tmlite.Verifier, error) {
	if trustNode {
		return nil, nil
	}

	if chainID == "" {
		return nil, errors.New("Invalid chainID")
	}
	if home == "" {
		return nil, errors.New("Invalid home")
	}
	if nodeURI == "" {
		return nil, errors.New("Invalid nodeURI")
	}

	node := rpcclient.NewHTTP(nodeURI, "/websocket")
	cacheSize := 10 // TODO: determine appropriate cache size
	verifier, err := tmliteProxy.NewVerifier(
		chainID, filepath.Join(home, ".gaialite"),
		node, log.NewNopLogger(), cacheSize,
	)

	if err != nil {
		return nil, err
	}

	return verifier, nil
}

// WithCodec returns a copy of the context with an updated codec.
func (ctx CLIContext) WithCodec(cdc *codec.Codec) CLIContext {
	ctx.Codec = cdc
	return ctx
}

// GetAccountDecoder gets the account decoder for auth.DefaultAccount.
func GetAccountDecoder(cdc *codec.Codec) auth.AccountDecoder {
	return func(accBytes []byte) (acct auth.Account, err error) {
		err = cdc.UnmarshalBinaryBare(accBytes, &acct)
		if err != nil {
			panic(err)
		}

		return acct, err
	}
}

// WithAccountDecoder returns a copy of the context with an updated account
// decoder.
func (ctx CLIContext) WithAccountDecoder(cdc *codec.Codec) CLIContext {
	ctx.AccDecoder = GetAccountDecoder(cdc)
	return ctx
}

// WithOutput returns a copy of the context with an updated output writer (e.g. stdout).
func (ctx CLIContext) WithOutput(w io.Writer) CLIContext {
	ctx.Output = w
	return ctx
}

// WithAccountStore returns a copy of the context with an updated AccountStore.
func (ctx CLIContext) WithAccountStore(accountStore string) CLIContext {
	ctx.AccountStore = accountStore
	return ctx
}

// WithFrom returns a copy of the context with an updated from address or name.
func (ctx CLIContext) WithFrom(from string) CLIContext {
	ctx.From = from
	return ctx
}

// WithTrustNode returns a copy of the context with an updated TrustNode flag.
func (ctx CLIContext) WithTrustNode(trustNode bool) CLIContext {
	ctx.TrustNode = trustNode
	return ctx
}

// WithNodeURI returns a copy of the context with an updated node URI.
func (ctx CLIContext) WithNodeURI(nodeURI string) CLIContext {
	ctx.NodeURI = nodeURI
	ctx.Client = rpcclient.NewHTTP(nodeURI, "/websocket")
	return ctx
}

// WithClient returns a copy of the context with an updated RPC client
// instance.
func (ctx CLIContext) WithClient(client rpcclient.Client) CLIContext {
	ctx.Client = client
	return ctx
}

// WithUseLedger returns a copy of the context with an updated UseLedger flag.
func (ctx CLIContext) WithUseLedger(useLedger bool) CLIContext {
	ctx.UseLedger = useLedger
	return ctx
}

// WithPassphrase returns a copy of the context with an passphrase for signing tx.
func (ctx CLIContext) WithPassphrase(passphrase string) CLIContext {
	ctx.Passphrase = passphrase
	return ctx
}

// WithVerifier - return a copy of the context with an updated Verifier
func (ctx CLIContext) WithVerifier(verifier tmlite.Verifier) CLIContext {
	ctx.Verifier = verifier
	return ctx
}

// WithGenerateOnly returns a copy of the context with updated GenerateOnly value
func (ctx CLIContext) WithGenerateOnly(generateOnly bool) CLIContext {
	ctx.GenerateOnly = generateOnly
	return ctx
}

// WithSimulation returns a copy of the context with updated Simulate value
func (ctx CLIContext) WithSimulation(simulate bool) CLIContext {
	ctx.Simulate = simulate
	return ctx
}

// WithFromName returns a copy of the context with an updated from account name.
func (ctx CLIContext) WithFromName(name string) CLIContext {
	ctx.FromName = name
	return ctx
}

// WithFromAddress returns a copy of the context with an updated from account
// address.
func (ctx CLIContext) WithFromAddress(addr sdk.AccAddress) CLIContext {
	ctx.FromAddress = addr
	return ctx
}

// WithBroadcastMode returns a copy of the context with an updated broadcast
// mode.
func (ctx CLIContext) WithBroadcastMode(mode string) CLIContext {
	ctx.BroadcastMode = mode
	return ctx
}

// PrintOutput prints output while respecting output and indent flags
// NOTE: pass in marshalled structs that have been unmarshaled
// because this function will panic on marshaling errors
func (ctx CLIContext) PrintOutput(toPrint fmt.Stringer) (err error) {
	var out []byte

	switch ctx.OutputFormat {
	case "text":
		out = []byte(toPrint.String())

	case "json":
		if ctx.Indent {
			out, err = ctx.Codec.MarshalJSONIndent(toPrint, "", "  ")
		} else {
			out, err = ctx.Codec.MarshalJSON(toPrint)
		}
	}

	if err != nil {
		return
	}

	fmt.Println(string(out))
	return
}

// GetFromFields returns a from account address and Keybase name given either
// an address or key name. If genOnly is true, only a valid Bech32 cosmos
// address is returned.
func GetFromFields(from string, genOnly bool, home string) (sdk.AccAddress, string, error) {
	if from == "" {
		return nil, "", nil
	}

	if genOnly {
		addr, err := sdk.AccAddressFromBech32(from)
		if err != nil {
			return nil, "", errors.Wrap(err, "must provide a valid Bech32 address for generate-only")
		}

		return addr, "", nil
	}

	keybase, err := keys.NewKeyBaseFromDir(home)
	if err != nil {
		return nil, "", err
	}

	var info cryptokeys.Info
	if addr, err := sdk.AccAddressFromBech32(from); err == nil {
		info, err = keybase.GetByAddress(addr)
		if err != nil {
			return nil, "", err
		}
	} else {
		info, err = keybase.Get(from)
		if err != nil {
			return nil, "", err
		}
	}

	return info.GetAddress(), info.GetName(), nil
}
