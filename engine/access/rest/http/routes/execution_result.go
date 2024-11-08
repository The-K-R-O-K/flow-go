package routes

import (
	"fmt"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/common"
	"github.com/onflow/flow-go/engine/access/rest/http/request"

	"github.com/onflow/flow-go/engine/access/rest/http/models"
)

// GetExecutionResultsByBlockIDs gets Execution Result payload by block IDs.
func GetExecutionResultsByBlockIDs(r *common.Request, backend access.API, link models.LinkGenerator) (interface{}, error) {
	req, err := request.GetExecutionResultByBlockIDsRequest(r)
	if err != nil {
		return nil, common.NewBadRequestError(err)
	}

	// for each block ID we retrieve execution result
	results := make([]models.ExecutionResult, len(req.BlockIDs))
	for i, id := range req.BlockIDs {
		res, err := backend.GetExecutionResultForBlockID(r.Context(), id)
		if err != nil {
			return nil, err
		}

		var response models.ExecutionResult
		err = response.Build(res, link)
		if err != nil {
			return nil, err
		}
		results[i] = response
	}

	return results, nil
}

// GetExecutionResultByID gets execution result by the ID.
func GetExecutionResultByID(r *common.Request, backend access.API, link models.LinkGenerator) (interface{}, error) {
	req, err := request.GetExecutionResultRequest(r)
	if err != nil {
		return nil, common.NewBadRequestError(err)
	}

	res, err := backend.GetExecutionResultByID(r.Context(), req.ID)
	if err != nil {
		return nil, err
	}

	if res == nil {
		err := fmt.Errorf("execution result with ID: %s not found", req.ID.String())
		return nil, common.NewNotFoundError(err.Error(), err)
	}

	var response models.ExecutionResult
	err = response.Build(res, link)
	if err != nil {
		return nil, err
	}

	return response, nil
}
