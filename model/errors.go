package model

import "errors"

// ErrDBNotInitialized indicates the SQL layer has not been prepared.
var ErrDBNotInitialized = errors.New("database is not initialized")

// ErrAISessionNotFound indicates the requested AI session does not exist.
var ErrAISessionNotFound = errors.New("AI session not found")
