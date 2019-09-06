module github.com/dgamingfoundation/dkglib

go 1.12

require (
	github.com/cosmos/cosmos-sdk v0.28.2-0.20190827131926-5aacf454e1b6
	github.com/dgamingfoundation/cosmos-utils/client v0.0.0-20190904115518-54b452a4037e
	github.com/dgamingfoundation/randapp v0.0.0-20190905141514-5b140ec96ded
	github.com/rcrowley/go-metrics v0.0.0-20190826022208-cac0b30c2563 // indirect
	github.com/rs/cors v1.7.0 // indirect
	github.com/tendermint/tendermint v0.32.3
)

replace golang.org/x/crypto => github.com/tendermint/crypto v0.0.0-20180820045704-3764759f34a5

replace github.com/tendermint/tendermint => github.com/dgamingfoundation/tendermint v0.27.4-0.20190904144504-a6a0f25fa2c3
