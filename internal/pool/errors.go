package pool

import "errors"

// ErrNoReadyModems indicates that no modems in the pool are in a ready state.
var ErrNoReadyModems = errors.New("no ready modems available in pool")

// ErrModemNotFound indicates that the requested modem ID does not exist in the pool.
var ErrModemNotFound = errors.New("modem not found in pool")

// ErrAllModemsFailed indicates that every candidate modem failed when executing the request.
var ErrAllModemsFailed = errors.New("all modems in pool failed to execute request")

// ErrInvalidStrategy indicates that the requested load balancing strategy is unknown or unsupported.
var ErrInvalidStrategy = errors.New("invalid pool strategy")
