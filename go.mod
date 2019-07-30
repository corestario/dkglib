module github.com/dgamingfoundation/dkglib

go 1.12

require (
	github.com/bgentry/speakeasy v0.1.0
	github.com/cosmos/cosmos-sdk v0.35.0
	github.com/dgamingfoundation/randapp v0.0.0-20190701100104-b2f0fa470065
	github.com/mattn/go-isatty v0.0.7
	github.com/mattn/go-runewidth v0.0.4 // indirect
	github.com/olekukonko/tablewriter v0.0.1
	github.com/pkg/errors v0.8.1
	github.com/spf13/afero v1.2.2 // indirect
	github.com/spf13/cobra v0.0.5
	github.com/spf13/viper v1.4.0
	github.com/stretchr/testify v1.3.0
	github.com/tendermint/go-amino v0.15.0
	github.com/tendermint/tendermint v0.31.7
	golang.org/x/sys v0.0.0-20190329044733-9eb1bfa1ce65 // indirect
	google.golang.org/appengine v1.4.0 // indirect
	google.golang.org/genproto v0.0.0-20190327125643-d831d65fe17d // indirect
)

replace github.com/cosmos/cosmos-sdk => github.com/cosmos/cosmos-sdk v0.0.0-20190625145210-5fb86c661ea1

replace golang.org/x/crypto => github.com/tendermint/crypto v0.0.0-20180820045704-3764759f34a5

replace github.com/tendermint/tendermint => github.com/dgamingfoundation/tendermint v0.27.4-0.20190604195457-d66632d1761e
