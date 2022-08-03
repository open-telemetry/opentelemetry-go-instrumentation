package errors

import "errors"

// ErrInterrupted can be used as an error to signal that a process was
// interrupted but didn't fail in any other way.
var ErrInterrupted = errors.New("interrupted")
var ErrProcessNotFound = errors.New("process_not_found")
var ErrABIWrongInstruction = errors.New("could not detect ABI, got wrong instruction")
