package validation

import (
	"fmt"
	"sort"
	"strings"
)

// Severity describes the publication impact of a validation result.
type Severity string

const (
	Error         Severity = "error"
	Warning       Severity = "warning"
	Informational Severity = "informational"
)

// Issue is the stable, machine-readable validation result contract. RecordID
// is "<file>" for file-level failures so errors always contain every field a
// release operator needs to locate the problem.
type Issue struct {
	Severity Severity `json:"severity"`
	Dataset  string   `json:"dataset"`
	File     string   `json:"file"`
	RecordID string   `json:"recordIdentifier"`
	Rule     string   `json:"rule"`
	Message  string   `json:"message"`
}

func (i Issue) String() string {
	return fmt.Sprintf("%s: dataset=%s file=%s record=%s rule=%s: %s",
		i.Severity, i.Dataset, i.File, i.RecordID, i.Rule, i.Message)
}

// Report contains every result without changing the data being inspected.
type Report struct {
	Issues []Issue `json:"issues"`
}

func (r *Report) add(severity Severity, dataset, file, recordID, rule, message string) {
	if dataset == "" {
		dataset = "release"
	}
	if file == "" {
		file = "<release>"
	}
	if recordID == "" {
		recordID = "<file>"
	}
	r.Issues = append(r.Issues, Issue{
		Severity: severity, Dataset: dataset, File: file, RecordID: recordID,
		Rule: rule, Message: message,
	})
}

func (r *Report) merge(other Report) { r.Issues = append(r.Issues, other.Issues...) }

func (r Report) HasErrors() bool {
	for _, issue := range r.Issues {
		if issue.Severity == Error {
			return true
		}
	}
	return false
}

func (r Report) Warnings() []string {
	result := make([]string, 0)
	for _, issue := range r.Issues {
		if issue.Severity == Warning {
			result = append(result, issue.String())
		}
	}
	return result
}

func (r *Report) sort() {
	sort.SliceStable(r.Issues, func(i, j int) bool {
		left, right := r.Issues[i], r.Issues[j]
		return fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s", left.File, left.RecordID, left.Rule, left.Severity, left.Message) <
			fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s", right.File, right.RecordID, right.Rule, right.Severity, right.Message)
	})
}

// Err returns only publication-blocking results. Warnings become blocking when
// warningsAsErrors is enabled by the caller.
func (r Report) Err(warningsAsErrors bool) error {
	blocking := make([]string, 0)
	for _, issue := range r.Issues {
		if issue.Severity == Error || warningsAsErrors && issue.Severity == Warning {
			blocking = append(blocking, issue.String())
		}
	}
	if len(blocking) == 0 {
		return nil
	}
	return fmt.Errorf("validation failed:\n%s", strings.Join(blocking, "\n"))
}
