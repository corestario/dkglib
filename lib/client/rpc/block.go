package rpc

import (
	"dgamingfoundation/dkglib/lib/client/context"
	"net/http"
	"strconv"

	"github.com/cosmos/cosmos-sdk/types/rest"
	"github.com/gorilla/mux"
	tmliteProxy "github.com/tendermint/tendermint/lite/proxy"
)

func getBlock(cliCtx context.CLIContext, height *int64) ([]byte, error) {
	// get the node
	node, err := cliCtx.GetNode()
	if err != nil {
		return nil, err
	}

	// header -> BlockchainInfo
	// header, tx -> Block
	// results -> BlockResults
	res, err := node.Block(height)
	if err != nil {
		return nil, err
	}

	if !cliCtx.TrustNode {
		check, err := cliCtx.Verify(res.Block.Height)
		if err != nil {
			return nil, err
		}

		err = tmliteProxy.ValidateBlockMeta(res.BlockMeta, check)
		if err != nil {
			return nil, err
		}

		err = tmliteProxy.ValidateBlock(res.Block, check)
		if err != nil {
			return nil, err
		}
	}

	if cliCtx.Indent {
		return cdc.MarshalJSONIndent(res, "", "  ")
	}
	return cdc.MarshalJSON(res)
}

// get the current blockchain height
func GetChainHeight(cliCtx context.CLIContext) (int64, error) {
	node, err := cliCtx.GetNode()
	if err != nil {
		return -1, err
	}
	status, err := node.Status()
	if err != nil {
		return -1, err
	}
	height := status.SyncInfo.LatestBlockHeight
	return height, nil
}

// REST

// REST handler to get a block
func BlockRequestHandlerFn(cliCtx context.CLIContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		height, err := strconv.ParseInt(vars["height"], 10, 64)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusBadRequest,
				"ERROR: Couldn't parse block height. Assumed format is '/block/{height}'.")
			return
		}
		chainHeight, err := GetChainHeight(cliCtx)
		if height > chainHeight {
			rest.WriteErrorResponse(w, http.StatusNotFound,
				"ERROR: Requested block height is bigger then the chain length.")
			return
		}
		output, err := getBlock(cliCtx, &height)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		rest.PostProcessResponse(w, cdc, output, cliCtx.Indent)
	}
}

// REST handler to get the latest block
func LatestBlockRequestHandlerFn(cliCtx context.CLIContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		output, err := getBlock(cliCtx, nil)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		rest.PostProcessResponse(w, cdc, output, cliCtx.Indent)
	}
}
