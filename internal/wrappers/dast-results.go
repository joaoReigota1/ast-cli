package wrappers

import "io"

type DastResultsWrapper interface {
	SendResults(body io.Reader) (*DastRiskCount, *WebError, error)
}
