package keys

import (
	"fmt"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/tendermint/tendermint/libs/bech32"
)

var bech32Prefixes = []string{
	sdk.Bech32PrefixAccAddr,
	sdk.Bech32PrefixAccPub,
	sdk.Bech32PrefixValAddr,
	sdk.Bech32PrefixValPub,
	sdk.Bech32PrefixConsAddr,
	sdk.Bech32PrefixConsPub,
}

type hexOutput struct {
	Human string `json:"human"`
	Bytes string `json:"bytes"`
}

func (ho hexOutput) String() string {
	return fmt.Sprintf("Human readable part: %v\nBytes (hex): %s", ho.Human, ho.Bytes)
}

func newHexOutput(human string, bs []byte) hexOutput {
	return hexOutput{Human: human, Bytes: fmt.Sprintf("%X", bs)}
}

type bech32Output struct {
	Formats []string `json:"formats"`
}

func newBech32Output(bs []byte) bech32Output {
	out := bech32Output{Formats: make([]string, len(bech32Prefixes))}
	for i, prefix := range bech32Prefixes {
		bech32Addr, err := bech32.ConvertAndEncode(prefix, bs)
		if err != nil {
			panic(err)
		}
		out.Formats[i] = bech32Addr
	}

	return out
}

func (bo bech32Output) String() string {
	out := make([]string, len(bo.Formats))

	for i, format := range bo.Formats {
		out[i] = fmt.Sprintf("  - %s", format)
	}

	return fmt.Sprintf("Bech32 Formats:\n%s", strings.Join(out, "\n"))
}
