package clob

import "errors"

var (
	ErrInvalidChainID    = errors.New("invalid chainID")
	ErrAuthUnavailableL1 = errors.New("a private key is needed to interact with this endpoint")
	ErrAuthUnavailableL2 = errors.New("API credentials are needed to interact with this endpoint")
)
