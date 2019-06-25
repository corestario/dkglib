package keys

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/olekukonko/tablewriter"

	"github.com/cosmos/cosmos-sdk/crypto/keys"
)

// available output formats.
const (
	OutputFormatText = "text"
	OutputFormatJSON = "json"

	// defaultKeyDBName is the client's subdirectory where keys are stored.
	defaultKeyDBName = "keys"
)

type bechKeyOutFn func(keyInfo keys.Info) (keys.KeyOutput, error)

// GetKeyInfo returns key info for a given name. An error is returned if the
// keybase cannot be retrieved or getting the info fails.
//func GetKeyInfo(name string, ctx context.CLIContext) (keys.Info, error) {
//	keybase, err := NewKeyBaseFromDir(ctx.Home)
//	if err != nil {
//		return nil, err
//	}
//
//	return keybase.Get(name)
//}

//// GetPassphrase returns a passphrase for a given name. It will first retrieve
//// the key info for that name if the type is local, it'll fetch input from
//// STDIN. Otherwise, an empty passphrase is returned. An error is returned if
//// the key info cannot be fetched or reading from STDIN fails.
//func GetPassphrase(name string, ctx context.CLIContext) (string, error) {
//	var passphrase string
//
//	keyInfo, err := GetKeyInfo(name, ctx)
//	if err != nil {
//		return passphrase, err
//	}
//
//	// we only need a passphrase for locally stored keys
//	// TODO: (ref: #864) address security concerns
//	if keyInfo.GetType() == keys.TypeLocal {
//		passphrase, err = ReadPassphraseFromStdin(name)
//		if err != nil {
//			return passphrase, err
//		}
//	}
//
//	return passphrase, nil
//}

// ReadPassphraseFromStdin attempts to read a passphrase from STDIN return an
// error upon failure.
//func ReadPassphraseFromStdin(name string) (string, error) {
//	buf := client.BufferStdin()
//	prompt := fmt.Sprintf("Password to sign with '%s':", name)
//
//	passphrase, err := client.GetPassword(prompt, buf)
//	if err != nil {
//		return passphrase, fmt.Errorf("Error reading passphrase: %v", err)
//	}
//
//	return passphrase, nil
//}

// NewKeyBaseFromDir initializes a keybase at a particular dir.
func NewKeyBaseFromDir(rootDir string) (keys.Keybase, error) {
	return getLazyKeyBaseFromDir(rootDir)
}

// NewInMemoryKeyBase returns a storage-less keybase.
func NewInMemoryKeyBase() keys.Keybase { return keys.NewInMemory() }

func getLazyKeyBaseFromDir(rootDir string) (keys.Keybase, error) {
	return keys.New(defaultKeyDBName, filepath.Join(rootDir, "keys")), nil
}

func printMultiSigKeyInfo(keyInfo keys.Info, bechKeyOut bechKeyOutFn) {
	ko, err := bechKeyOut(keyInfo)
	if err != nil {
		panic(err)
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"WEIGHT", "THRESHOLD", "ADDRESS", "PUBKEY"})
	threshold := fmt.Sprintf("%d", ko.Threshold)
	for _, pk := range ko.PubKeys {
		weight := fmt.Sprintf("%d", pk.Weight)
		table.Append([]string{weight, threshold, pk.Address, pk.PubKey})
	}
	table.Render()
}
