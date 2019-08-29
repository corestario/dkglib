module github.com/dgamingfoundation/dkglib

go 1.12

require (
	bou.ke/monkey v1.0.1 // indirect
	github.com/VividCortex/gohistogram v1.0.0 // indirect
	github.com/cosmos/cosmos-sdk v0.37.0
	github.com/cosmos/tools v0.0.0-20190729191304-444fa9c55188 // indirect
	github.com/dgamingfoundation/cosmos-utils/client v0.0.0-20190829124036-5189e32ac7d3
	github.com/dgamingfoundation/randapp v0.0.0-20190715103154-e4b398ef456d
	github.com/otiai10/copy v0.0.0-20180813032824-7e9a647135a1 // indirect
	github.com/otiai10/curr v0.0.0-20150429015615-9b4961190c95 // indirect
	github.com/otiai10/mint v1.2.3 // indirect
	github.com/rakyll/statik v0.1.6 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20190826022208-cac0b30c2563 // indirect
	github.com/rs/cors v1.7.0 // indirect
	github.com/spf13/viper v1.4.0
	github.com/tendermint/crypto v0.0.0-20190823183015-45b1026d81ae // indirect
	github.com/tendermint/tendermint v0.32.2
)

replace golang.org/x/crypto => github.com/tendermint/crypto v0.0.0-20180820045704-3764759f34a5

replace github.com/tendermint/tendermint => github.com/dgamingfoundation/tendermint v0.27.4-0.20190703072235-fcd273ff19b0
