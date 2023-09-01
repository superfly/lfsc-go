package lfsc

import (
	"fmt"
	"time"
)

// Cluster represents a grouping of databases in LiteFS Cloud.
type Cluster struct {
	OrgID     int       `json:"orgID"`
	Name      string    `json:"name"`
	Region    string    `json:"region"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Error represents an error code & message returned from LiteFS Cloud.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"error"`
}

// Error returns a string-formatted message. Implements the error interface.
func (e *Error) Error() string {
	return fmt.Sprintf("%s [%s]", e.Message, e.Code)
}
