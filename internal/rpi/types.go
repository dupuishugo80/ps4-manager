// Package rpi is a typed client for the PS4 Remote Package Installer HTTP API
// exposed by GoldHEN on port 12800.
package rpi

import (
	"encoding/json"
	"fmt"
)

const DefaultPort = 12800

type PackageType string

const (
	PackageTypeDirect    PackageType = "direct"
	PackageTypeRefPkgURL PackageType = "ref_pkg_url"
)

type SubType int

const (
	SubTypeGame    SubType = 6
	SubTypeAC      SubType = 7
	SubTypePatch   SubType = 8
	SubTypeLicense SubType = 9
)

type InstallRequest struct {
	Type     PackageType `json:"type"`
	Packages []string    `json:"packages,omitempty"`
	URL      string      `json:"url,omitempty"`
}

type InstallResponse struct {
	TaskID int64  `json:"task_id"`
	Title  string `json:"title"`
}

type IsExistsResponse struct {
	Exists string `json:"exists"`
	Size   uint64 `json:"size,omitempty"`
}

func (r IsExistsResponse) Found() bool { return r.Exists == "true" }

type FindTaskResponse struct {
	TaskID int64 `json:"task_id"`
}

type TaskProgress struct {
	Bits             uint32 `json:"bits"`
	Error            int32  `json:"error"`
	Length           uint64 `json:"length"`
	Transferred      uint64 `json:"transferred"`
	LengthTotal      uint64 `json:"length_total"`
	TransferredTotal uint64 `json:"transferred_total"`
	NumIndex         uint32 `json:"num_index"`
	NumTotal         uint32 `json:"num_total"`
	RestSec          uint32 `json:"rest_sec"`
	RestSecTotal     uint32 `json:"rest_sec_total"`
	PreparingPercent int32  `json:"preparing_percent"`
	LocalCopyPercent int32  `json:"local_copy_percent"`
}

// APIError wraps a {"status":"fail", ...} response from the RPI service.
type APIError struct {
	Message   string
	ErrorCode uint32
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("rpi: %s", e.Message)
	}
	return fmt.Sprintf("rpi: error_code=0x%08X", e.ErrorCode)
}

// statusEnvelope inspects the shared {status, error, error_code} shape without
// committing to a type for `error`: it is a string on failures and an int on
// get_task_progress success responses.
type statusEnvelope struct {
	Status    string          `json:"status"`
	Error     json.RawMessage `json:"error"`
	ErrorCode uint32          `json:"error_code"`
}
