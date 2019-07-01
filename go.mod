module dgamingfoundation/dkglib

go 1.12

require (
	github.com/bartekn/go-bip39 v0.0.0-20171116152956-a05967ea095d
	github.com/bgentry/speakeasy v0.1.0
	github.com/cosmos/cosmos-sdk v0.35.0
	github.com/cosmos/go-bip39 v0.0.0-20180618194314-52158e4697b8
	github.com/cosmos/sdk-application-tutorial v0.0.0-20190624153636-5544cb2b56cc // indirect
	github.com/dgamingfoundation/randapp v0.0.0-20190628131059-530455b10c7e
	github.com/gorilla/mux v1.7.0
	github.com/mattn/go-isatty v0.0.7
	github.com/mattn/go-runewidth v0.0.4 // indirect
	github.com/olekukonko/tablewriter v0.0.1
	github.com/pelletier/go-toml v1.2.0
	github.com/pkg/errors v0.8.1
	github.com/rakyll/statik v0.1.4
	github.com/spf13/cobra v0.0.5
	github.com/spf13/viper v1.4.0
	github.com/tendermint/go-amino v0.15.0
	github.com/tendermint/tendermint v0.31.6
	google.golang.org/appengine v1.4.0 // indirect
)

replace github.com/cosmos/cosmos-sdk => github.com/cosmos/cosmos-sdk v0.28.2-0.20190616100639-18415eedaf25

replace golang.org/x/crypto => github.com/tendermint/crypto v0.0.0-20180820045704-3764759f34a5

replace github.com/tendermint/tendermint => github.com/dgamingfoundation/tendermint v0.27.4-0.20190624123441-28dd49477e0b
