package rpc

import (
	"log"
	"net/http"
	"strconv"

	ctypes "github.com/tendermint/tendermint/rpc/core/types"

	"dgamingfoundation/dkglib/lib/client/context"
	"github.com/cosmos/cosmos-sdk/types/rest"
)

func getNodeStatus(cliCtx context.CLIContext) (*ctypes.ResultStatus, error) {
	// get the node
	node, err := cliCtx.GetNode()
	if err != nil {
		return &ctypes.ResultStatus{}, err
	}

	return node.Status()
}

// REST

// REST handler for node info
func NodeInfoRequestHandlerFn(cliCtx context.CLIContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status, err := getNodeStatus(cliCtx)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		nodeInfo := status.NodeInfo
		rest.PostProcessResponse(w, cdc, nodeInfo, cliCtx.Indent)
	}
}

// REST handler for node syncing
func NodeSyncingRequestHandlerFn(cliCtx context.CLIContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status, err := getNodeStatus(cliCtx)
		if err != nil {
			rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		syncing := status.SyncInfo.CatchingUp
		if _, err := w.Write([]byte(strconv.FormatBool(syncing))); err != nil {
			log.Printf("could not write response: %v", err)
		}
	}
}
