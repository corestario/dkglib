## DKGLib

Implements on-chain logic of DKG protocol described in "Secure Distributed Key Generation for Discrete-Log Based Cryptosystems" by R. Gennaro, S. Jarecki, H. Krawczyk, and T. Rabin.

### How it works

Library communicates with Cosmos Application called [RandApp](https://github.com/dgamingfoundation/randapp)

Transactions sent by light blockchain client (copy of Cosmos CliContext and TxBuilder but with explicit configuration)

#### How to use
```go
// CLIContext implements a typical CLI context created in SDK modules for
// transaction handling and queries
// DKGLib uses own Context with explicit configuration
cliCtx, err := cliCTX.NewCLIContext(chainID, nodeEndpoint, validatorName, genOnly, broadcastMode, vfrHome, height, trustNode, cliHome, "")
if err != nil {
    return nil, nil, err
}
cliCtx = cliCtx.WithCodec(cdc).WithAccountDecoder(cdc)
accNumber, err := cliCtx.GetAccountNumber(cliCtx.FromAddress)
if err != nil {
    return nil, nil, err
}
kb, err := keys.NewKeyBaseFromDir(cliCtx.Home)
if err != nil {
    return nil, nil, err
}
// TxBuilder implements tx generation
txBldr := authtxb.NewTxBuilder(utils.GetTxEncoder(cdc), accNumber, 0, 0, 0.0, false, cliCtx.Verifier.ChainID(), "", nil, nil).WithKeybase(kb)
if err := cliCtx.EnsureAccountExists(); err != nil {
    return nil, nil, fmt.Errorf("failed to find account: %v", err)
}

// Create DKGClient
oc := lib.NewOnChainDKG(cli, txBldr)

// Start new round
if err := oc.StartRound(types.NewValidatorSet(vals), pval, mockF, logger, 0); err != nil {
    panic(fmt.Sprintf("failed to start round: %v", err))
}

// Process blocks
tk := time.NewTicker(time.Second)
for {
    select {
    case <-tk.C:
        if err, ok := oc.ProcessBlock(); err != nil {
            panic(fmt.Sprintf("failed to start round: %v", err))
        } else if ok {
            wg.Done()
            return
        }
    }
}
```