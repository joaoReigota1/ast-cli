package wrappers

import (
	"encoding/json"
	commonParams "github.com/checkmarx/ast-cli/internal/params"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"io"
	"net/http"
)

type DastResultsHTTPWrapper struct {
	path string
}

func NewHTTPDastResultsWrapper(path string) *DastResultsHTTPWrapper {
	return &DastResultsHTTPWrapper{
		path: path,
	}
}

func (r *DastResultsHTTPWrapper) SendResults(body io.Reader) (*DastRiskCount, *WebError, error) {
	clientTimeout := viper.GetUint(commonParams.ClientTimeoutKey)
	resp, err := SendPrivateHTTPRequestWithQueryParams(http.MethodPost, r.path, make(map[string]string), body, clientTimeout)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)

	switch resp.StatusCode {
	case http.StatusBadRequest, http.StatusInternalServerError:
		errorModel := WebError{}
		err = decoder.Decode(&errorModel)
		if err != nil {
			return nil, nil, errors.Wrapf(err, failedToParseGetResults)
		}
		return nil, &errorModel, nil
	case http.StatusOK:
		model := DastRiskCount{}
		err = decoder.Decode(&model)
		if err != nil {
			return nil, nil, errors.Wrapf(err, failedToParseGetResults)
		}
		return &model, nil, nil
	default:
		return nil, nil, errors.Errorf("response status code %d", resp.StatusCode)
	}
}
