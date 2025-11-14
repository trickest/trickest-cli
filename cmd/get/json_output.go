package get

import (
	"time"

	"github.com/google/uuid"
	"github.com/trickest/trickest-cli/pkg/stats"
	"github.com/trickest/trickest-cli/pkg/trickest"
)

// JSONRun represents a simplified version of a Run for JSON output
type JSONRun struct {
	ID uuid.UUID `json:"id"`

	Status string `json:"status"`

	Author       string `json:"author"`
	CreationType string `json:"creation_type"`

	CreatedDate   *time.Time `json:"created_date"`
	StartedDate   *time.Time `json:"started_date"`
	CompletedDate *time.Time `json:"completed_date"`

	Duration        trickest.Duration `json:"duration"`
	AverageDuration trickest.Duration `json:"average_duration,omitempty"`

	WorkflowName        string    `json:"workflow_name"`
	WorkflowInfo        uuid.UUID `json:"workflow_info"`
	WorkflowVersionInfo uuid.UUID `json:"workflow_version_info"`

	Fleet        uuid.UUID `json:"fleet"`
	FleetName    string    `json:"fleet_name"`
	UseStaticIPs bool      `json:"use_static_ips"`
	Machines     int       `json:"machines"`
	Parallelism  int       `json:"parallelism"`
	IPAddresses  []string  `json:"ip_addresses"`

	RunInsights *trickest.RunSubJobInsights `json:"run_insights,omitempty"`
	SubJobs     []JSONSubJob                `json:"subjobs"`
}

// JSONSubJob represents a simplified version of a SubJob for JSON output
type JSONSubJob struct {
	Label string `json:"label,omitempty"`
	Name  string `json:"name,omitempty"`

	Status  string `json:"status"`
	Message string `json:"message,omitempty"`

	StartedDate  *time.Time        `json:"started_date,omitempty"`
	FinishedDate *time.Time        `json:"finished_date,omitempty"`
	Duration     trickest.Duration `json:"duration,omitempty"`

	IPAddress string `json:"ip_address,omitempty"`

	TaskGroup      bool                  `json:"task_group,omitempty"`
	TaskCount      int                   `json:"task_count,omitempty"`
	Children       []JSONSubJob          `json:"children,omitempty"`
	TaskIndex      int                   `json:"task_index,omitempty"`
	TaskGroupStats *stats.TaskGroupStats `json:"task_group_stats,omitempty"`
}

// NewJSONRun creates a new JSONRun from a trickest.Run
func NewJSONRun(run *trickest.Run, subjobs []trickest.SubJob, taskGroupStatsMap map[uuid.UUID]stats.TaskGroupStats) *JSONRun {
	jsonRun := &JSONRun{
		ID:                  *run.ID,
		Status:              run.Status,
		Author:              run.Author,
		CreationType:        run.CreationType,
		CreatedDate:         run.CreatedDate,
		StartedDate:         run.StartedDate,
		CompletedDate:       run.CompletedDate,
		AverageDuration:     *run.AverageDuration,
		WorkflowName:        run.WorkflowName,
		WorkflowInfo:        *run.WorkflowInfo,
		WorkflowVersionInfo: *run.WorkflowVersionInfo,
		Fleet:               *run.Fleet,
		FleetName:           run.FleetName,
		UseStaticIPs:        *run.UseStaticIPs,
		IPAddresses:         run.IPAddresses,
		RunInsights:         run.RunInsights,
		Machines:            run.Machines,
		Parallelism:         run.Parallelism,
		Duration:            trickest.Duration{Duration: run.Duration()},
	}

	jsonRun.SubJobs = make([]JSONSubJob, len(subjobs))
	for i, subjob := range subjobs {
		jsonRun.SubJobs[i] = *NewJSONSubJob(&subjob, taskGroupStatsMap)
	}

	return jsonRun
}

// NewJSONSubJob creates a new JSONSubJob from a trickest.SubJob
func NewJSONSubJob(subjob *trickest.SubJob, taskGroupStats map[uuid.UUID]stats.TaskGroupStats) *JSONSubJob {
	jsonSubJob := &JSONSubJob{
		Label:        subjob.Label,
		Name:         subjob.Name,
		Status:       subjob.Status,
		Message:      subjob.Message,
		StartedDate:  &subjob.StartedDate,
		FinishedDate: &subjob.FinishedDate,
		IPAddress:    subjob.IPAddress,
		TaskGroup:    subjob.TaskGroup,
		TaskIndex:    subjob.TaskIndex,
	}

	if !subjob.StartedDate.IsZero() {
		if subjob.FinishedDate.IsZero() {
			jsonSubJob.FinishedDate = nil
			jsonSubJob.Duration = trickest.Duration{Duration: time.Since(subjob.StartedDate)}
		} else {
			jsonSubJob.Duration = trickest.Duration{Duration: subjob.FinishedDate.Sub(subjob.StartedDate)}
		}
	} else {
		jsonSubJob.StartedDate = nil
	}

	if len(subjob.Children) > 0 {
		jsonSubJob.TaskCount = len(subjob.Children)
	}

	if subjob.TaskGroup && taskGroupStats != nil && stats.HasInterestingStats(subjob.Name) {
		stats, ok := taskGroupStats[subjob.ID]
		if ok {
			jsonSubJob.TaskGroupStats = &stats
		}
		for _, child := range subjob.Children {
			jsonSubJob.Children = append(jsonSubJob.Children, *NewJSONSubJob(&child, nil))
		}
	}

	return jsonSubJob
}
