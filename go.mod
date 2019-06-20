module dgamingfoundation/dkglib

go 1.12

require (
	github.com/bgentry/speakeasy v0.1.0
	github.com/cosmos/cosmos-sdk v0.34.7
	github.com/dgamingfoundation/randapp v0.0.0-00010101000000-000000000000
	github.com/mattn/go-isatty v0.0.6
	github.com/mattn/go-runewidth v0.0.4 // indirect
	github.com/olekukonko/tablewriter v0.0.1 // indirect
	github.com/pelletier/go-toml v1.2.0
	github.com/pkg/errors v0.8.1
	github.com/spf13/cobra v0.0.4
	github.com/spf13/viper v1.4.0
	github.com/tendermint/go-amino v0.15.0
	github.com/tendermint/tendermint v0.31.7
)

replace github.com/dgamingfoundation/randapp => /Users/pr0n00gler/go/src/github.com/dgamingfoundation/randapp

replace golang.org/x/crypto => github.com/tendermint/crypto v0.0.0-20180820045704-3764759f34a5

replace github.com/tendermint/tendermint => /Users/pr0n00gler/go/src/github.com/tendermint/tendermint
