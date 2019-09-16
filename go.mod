module github.com/dgamingfoundation/dkglib

go 1.12

require (
	github.com/cosmos/cosmos-sdk v0.28.2-0.20190827131926-5aacf454e1b6
	github.com/dgamingfoundation/cosmos-utils/client v0.0.0-20190904115518-54b452a4037e
	github.com/dgamingfoundation/tendermint v0.27.4-0.20190904144504-a6a0f25fa2c3
	github.com/pkg/errors v0.8.1
	github.com/tendermint/go-amino v0.15.0
	go.dedis.ch/kyber/v3 v3.0.4
)

replace golang.org/x/crypto => github.com/tendermint/crypto v0.0.0-20180820045704-3764759f34a5
