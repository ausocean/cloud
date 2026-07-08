package broadcast

import "errors"

// WarnSkipShutdown is a pseudo-error which represents that shutdown was skipped.
var WarnSkipShutdown = errors.New("shutdown set to skip")
