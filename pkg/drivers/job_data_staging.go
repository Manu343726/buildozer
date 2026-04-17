// Package drivers provides driver utilities.
// JobDataStager is re-exported from pkg/staging for backward compatibility.
package drivers

import (
	"github.com/Manu343726/buildozer/pkg/staging"
)

// JobDataStager is re-exported from staging for backward compatibility
type JobDataStager = staging.JobDataStager

// JobDataMode is re-exported from staging for backward compatibility
type JobDataMode = staging.JobDataMode

// JobDataMode constants
const (
	JobDataModeReference = staging.JobDataModeReference
	JobDataModeContent   = staging.JobDataModeContent
)

// VerificationMode is re-exported from staging for backward compatibility
type VerificationMode = staging.VerificationMode

// VerificationMode constants
const (
	VerificationModeSaved     = staging.VerificationModeSaved
	VerificationModeIntegrity = staging.VerificationModeIntegrity
)

// NewJobDataStager is re-exported from staging for backward compatibility
var NewJobDataStager = staging.NewJobDataStager

// ComputeFileHash is re-exported from staging for backward compatibility
var ComputeFileHash = staging.ComputeFileHash

// ComputeContentHash is re-exported from staging for backward compatibility
var ComputeContentHash = staging.ComputeContentHash
