module github.com/corestario/dkglib

go 1.12

require (
	github.com/corestario/cosmos-utils/client v0.0.0-20191209221021-bc64f205ca9b
	github.com/cosmos/cosmos-sdk v0.28.2-0.20190827131926-5aacf454e1b6
	github.com/tendermint/go-amino v0.15.1
	github.com/tendermint/tendermint v0.32.8
	go.dedis.ch/kyber/v3 v3.0.9
)

replace golang.org/x/crypto => github.com/tendermint/crypto v0.0.0-20180820045704-3764759f34a5

replace github.com/tendermint/tendermint => ./../tendermint

replace github.com/cosmos/cosmos-sdk => ./../cosmos-sdk

replace github.com/corestario/cosmos-utils/client => ./../cosmos-utils/client

replace github.com/corestario/dkglib => ./
