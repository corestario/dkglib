module github.com/dgamingfoundation/dkglib

go 1.12

require (
	github.com/VividCortex/gohistogram v1.0.0 // indirect
	github.com/cosmos/cosmos-sdk v0.28.2-0.20190827131926-5aacf454e1b6
	github.com/dgamingfoundation/cosmos-utils/client v0.0.0-20191105143510-3ddb0501dfbd
	github.com/dgamingfoundation/tendermint v0.27.3
	github.com/stumble/gorocksdb v0.0.3 // indirect
	github.com/tendermint/go-amino v0.15.1
	github.com/tendermint/tendermint v0.32.6
	go.dedis.ch/kyber/v3 v3.0.4
)

replace golang.org/x/crypto => github.com/tendermint/crypto v0.0.0-20180820045704-3764759f34a5

replace github.com/tendermint/tendermint => ../tendermint
