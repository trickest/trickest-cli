package investigate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/google/uuid"
	"github.com/trickest/trickest-cli/pkg/config"
	display "github.com/trickest/trickest-cli/pkg/display/run"
	"github.com/trickest/trickest-cli/pkg/trickest"
	"github.com/trickest/trickest-cli/util"

	"github.com/spf13/cobra"
)

// Config holds the configuration for the investigate command
type Config struct {
	Token   string
	BaseURL string

	RunID   string
	RunSpec config.WorkflowRunSpec

	from string
	to   string

	JSONOutput bool
}

var cfg = &Config{}

func init() {
	InvestigateCmd.Flags().StringVar(&cfg.RunID, "run", "", "Investigate a specific run")
	InvestigateCmd.Flags().StringVar(&cfg.from, "from", "", "Start time of the investigation period (defaults to run's start time; supported formats: 2006-01-02 15:04:05, 15:04:05, 15:04, 3:04PM)")
	InvestigateCmd.Flags().StringVar(&cfg.to, "to", "", "End time of the investigation period (defaults to current time; supported formats: 2006-01-02 15:04:05, 15:04:05, 15:04, 3:04PM)")
	InvestigateCmd.Flags().BoolVar(&cfg.JSONOutput, "json", false, "Display output in JSON format")
}

// GetCmd represents the get command
var InvestigateCmd = &cobra.Command{
	Use:   "investigate",
	Short: "Investigate a workflow run's execution details within a specific time range",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		cfg.Token = util.GetToken()
		cfg.BaseURL = util.Cfg.BaseUrl
		cfg.RunSpec = config.WorkflowRunSpec{
			RunID:        cfg.RunID,
			SpaceName:    util.SpaceName,
			ProjectName:  util.ProjectName,
			WorkflowName: util.WorkflowName,
			URL:          util.URL,
		}

		if err := run(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

// Investigation represents the complete investigation data
type Investigation struct {
	InvestigationTimeRange TimeRange    `json:"investigation_time_range"`
	WorkflowName           string       `json:"workflow_name"`
	Author                 string       `json:"author"`
	RunID                  uuid.UUID    `json:"run_id"`
	WorkflowID             uuid.UUID    `json:"workflow_id"`
	RunURL                 string       `json:"url"`
	IPAddresses            []string     `json:"ip_addresses"`
	SubJobs                []SubJobInfo `json:"subjobs"`
}

// SubJobInfo represents a processed subjob with all relevant information
type SubJobInfo struct {
	Label     string    `json:"label"`
	Name      string    `json:"name"`
	IPAddress string    `json:"ip_address"`
	URL       string    `json:"url"`
	Status    string    `json:"status"`
	Duration  Duration  `json:"duration"`
	Start     LocalTime `json:"start"`
	End       LocalTime `json:"end"`
	TaskIndex int       `json:"task_index,omitempty"`
}

// LocalTime is a custom type for time that json marshals to local time
type LocalTime struct {
	time time.Time
}

func (lt LocalTime) MarshalJSON() ([]byte, error) {
	localTime := lt.time.Local()
	return json.Marshal(localTime.Format(time.RFC1123))
}

// Duration is a custom type for duration that json marshals to "hh:mm:ss" matching the web UI
type Duration struct {
	Duration time.Duration
}

func (d *Duration) MarshalJSON() ([]byte, error) {
	seconds := int64(d.Duration.Seconds())
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60
	return json.Marshal(fmt.Sprintf("%02d:%02d:%02d", hours, minutes, secs))
}

// TimeRange represents a time interval with start and end times
type TimeRange struct {
	Start LocalTime `json:"start"`
	End   LocalTime `json:"end"`
}

// Overlaps returns true if the time range overlaps with another time range
func (tr TimeRange) Overlaps(other TimeRange) bool {
	// Compare in UTC for consistency
	return !tr.Start.time.After(other.End.time) && !tr.End.time.Before(other.Start.time)
}

// SubJobTimeRange returns the time range for a subjob
// If the subjob has not finished, it will use the end of the time range as the end time
func SubJobTimeRange(subJob trickest.SubJob, rangeEnd time.Time) TimeRange {
	end := rangeEnd
	if !subJob.FinishedDate.IsZero() {
		end = subJob.FinishedDate
	}
	return TimeRange{
		Start: LocalTime{time: subJob.StartedDate},
		End:   LocalTime{time: end},
	}
}

func generateInvestigationMarkdown(investigation Investigation) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# %s\n\n", investigation.WorkflowName))

	sb.WriteString("## Investigation Time Range\n\n")
	sb.WriteString(fmt.Sprintf("`%s` to `%s`\n\n",
		investigation.InvestigationTimeRange.Start.time.Local().Format(time.DateTime),
		investigation.InvestigationTimeRange.End.time.Local().Format(time.DateTime)),
	)

	if len(investigation.IPAddresses) > 0 {
		sb.WriteString("---\n\n")
		sb.WriteString("## IP Addresses\n\n")
		for _, ip := range investigation.IPAddresses {
			sb.WriteString(fmt.Sprintf("- `%s`\n", ip))
		}
		sb.WriteString("---\n\n")
	}

	sb.WriteString("## Tasks\n\n")

	if len(investigation.SubJobs) == 0 {
		sb.WriteString("No tasks were executed in the specified time range.\n\n")
	} else {
		for _, sj := range investigation.SubJobs {
			var header string
			if sj.TaskIndex != 0 {
				header = fmt.Sprintf("%s (%s) [%d]", sj.Label, sj.Name, sj.TaskIndex)
			} else {
				header = fmt.Sprintf("%s (%s)", sj.Label, sj.Name)
			}

			sb.WriteString(generateSubJobMarkdown(header, sj))
			sb.WriteString("---\n\n")
		}
	}

	return sb.String()
}

func generateSubJobMarkdown(header string, sj SubJobInfo) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### %s\n\n", header))
	sb.WriteString(fmt.Sprintf("[View in editor](%s)\n\n", sj.URL))
	sb.WriteString(fmt.Sprintf("- IP Address: `%s`\n", sj.IPAddress))
	sb.WriteString(fmt.Sprintf("- Duration: `%s` (started: `%s`, finished: `%s`)\n",
		display.FormatDuration(sj.Duration.Duration),
		sj.Start.time.Local().Format(time.TimeOnly),
		sj.End.time.Local().Format(time.TimeOnly)))

	return sb.String()
}

func run(cfg *Config) error {
	client, err := trickest.NewClient(
		trickest.WithToken(cfg.Token),
		trickest.WithBaseURL(cfg.BaseURL),
	)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	ctx := context.Background()

	runs, err := cfg.RunSpec.GetRuns(ctx, client)
	if err != nil {
		return fmt.Errorf("failed to get run: %w", err)
	}
	if len(runs) != 1 {
		return fmt.Errorf("expected 1 run, got %d", len(runs))
	}
	run := runs[0]

	if run.StartedDate == nil {
		return fmt.Errorf("the run has not started yet")
	}

	var investigationRange TimeRange
	if cfg.from != "" {
		fromTime, err := parseTime(cfg.from, run.StartedDate)
		if err != nil {
			return fmt.Errorf("failed to parse from time: %w", err)
		}
		investigationRange.Start = LocalTime{time: fromTime}
	} else {
		investigationRange.Start = LocalTime{time: *run.StartedDate}
	}

	if cfg.to != "" {
		referenceDate := run.CompletedDate
		if referenceDate == nil {
			referenceDate = run.StartedDate
		}
		toTime, err := parseTime(cfg.to, referenceDate)
		if err != nil {
			return fmt.Errorf("failed to parse to time: %w", err)
		}
		investigationRange.End = LocalTime{time: toTime}
	} else {
		investigationRange.End = LocalTime{time: time.Now()}
	}

	if investigationRange.Start.time.After(investigationRange.End.time) {
		return fmt.Errorf("invalid time range: start time (%s) is after end time (%s)",
			investigationRange.Start.time.Format(time.DateTime),
			investigationRange.End.time.Format(time.DateTime))
	}

	subJobs, err := client.GetSubJobs(ctx, *run.ID)
	if err != nil {
		return fmt.Errorf("failed to get sub jobs: %w", err)
	}

	var filteredSubJobs []trickest.SubJob
	for _, subJob := range subJobs {
		if subJob.StartedDate.IsZero() {
			continue
		}
		subJobRange := SubJobTimeRange(subJob, investigationRange.End.time)
		if investigationRange.Overlaps(subJobRange) {
			filteredSubJobs = append(filteredSubJobs, subJob)
		}
	}

	for i := range filteredSubJobs {
		if filteredSubJobs[i].TaskGroup {
			childSubJobs, err := client.GetChildSubJobs(ctx, filteredSubJobs[i].ID)
			if err != nil {
				return fmt.Errorf("failed to get child sub jobs: %w", err)
			}
			for _, child := range childSubJobs {
				if child.StartedDate.IsZero() {
					continue
				}
				childRange := SubJobTimeRange(child, investigationRange.End.time)
				if investigationRange.Overlaps(childRange) {
					filteredSubJobs[i].Children = append(filteredSubJobs[i].Children, child)
				}
			}
		}
	}

	workflowVersion, err := client.GetWorkflowVersion(ctx, *run.WorkflowVersionInfo)
	if err != nil {
		return fmt.Errorf("failed to get workflow version: %w", err)
	}
	filteredSubJobs = trickest.LabelSubJobs(filteredSubJobs, *workflowVersion)

	investigation := Investigation{
		RunID:                  *run.ID,
		WorkflowID:             *run.WorkflowInfo,
		RunURL:                 constructRunURL(*run.WorkflowInfo, *run.ID),
		InvestigationTimeRange: investigationRange,
		WorkflowName:           run.WorkflowName,
		Author:                 run.Author,
	}

	// Flatten task group children, convert to SubJobInfo structs, and collect unique IP addresses
	ipAddresses := make(map[string]struct{})
	for _, subJob := range filteredSubJobs {
		if subJob.TaskGroup {
			for _, child := range subJob.Children {
				if child.IPAddress != "" {
					ipAddresses[child.IPAddress] = struct{}{}
				}
				investigation.SubJobs = append(investigation.SubJobs, SubJobInfo{
					Label:     subJob.Label,
					Name:      subJob.Name,
					URL:       constructNodeURL(investigation.RunURL, subJob.Name),
					IPAddress: child.IPAddress,
					Status:    strings.ToLower(child.Status),
					Duration:  Duration{Duration: child.FinishedDate.Sub(child.StartedDate)},
					Start:     LocalTime{time: child.StartedDate},
					End:       LocalTime{time: child.FinishedDate},
					TaskIndex: child.TaskIndex,
				})
			}
		} else {
			if subJob.IPAddress != "" {
				ipAddresses[subJob.IPAddress] = struct{}{}
			}
			investigation.SubJobs = append(investigation.SubJobs, SubJobInfo{
				Label:     subJob.Label,
				Name:      subJob.Name,
				URL:       constructNodeURL(investigation.RunURL, subJob.Name),
				IPAddress: subJob.IPAddress,
				Status:    strings.ToLower(subJob.Status),
				Duration:  Duration{Duration: subJob.FinishedDate.Sub(subJob.StartedDate)},
				Start:     LocalTime{time: subJob.StartedDate},
				End:       LocalTime{time: subJob.FinishedDate},
			})
		}
	}
	investigation.IPAddresses = make([]string, 0, len(ipAddresses))
	for ip := range ipAddresses {
		investigation.IPAddresses = append(investigation.IPAddresses, ip)
	}
	slices.Sort(investigation.IPAddresses)

	// Sort subjobs by start time
	sort.Slice(investigation.SubJobs, func(i, j int) bool {
		if investigation.SubJobs[i].Start.time.Equal(investigation.SubJobs[j].Start.time) {
			return investigation.SubJobs[i].End.time.Before(investigation.SubJobs[j].End.time)
		}
		return investigation.SubJobs[i].Start.time.Before(investigation.SubJobs[j].Start.time)
	})

	if cfg.JSONOutput {
		investigationJSON, err := json.MarshalIndent(investigation, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal investigation to JSON: %w", err)
		}
		fmt.Println(string(investigationJSON))
	} else {
		investigationMarkdown := generateInvestigationMarkdown(investigation)
		r, _ := glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(-1),
		)
		out, err := r.Render(investigationMarkdown)
		if err != nil {
			return fmt.Errorf("failed to render investigation output: %w", err)
		}
		fmt.Println(out)
	}

	return nil
}

func parseTime(timeStr string, referenceDate *time.Time) (time.Time, error) {
	if timeStr == "" {
		return time.Time{}, nil
	}

	if strings.Contains(timeStr, " ") {
		t, err := time.Parse(time.DateTime, timeStr)
		if err == nil {
			return t, nil
		}
	}

	// Try time-only formats and infer the date from the reference date
	formats := []string{
		"15:04",
		time.TimeOnly, // 15:04:05
		time.Kitchen,  // 3:04PM
	}

	var parseErr error
	for _, format := range formats {
		parsedTime, err := time.Parse(format, timeStr)
		if err == nil {
			if referenceDate == nil {
				return time.Time{}, fmt.Errorf("reference date is required for partial time format")
			}

			hour, min, sec := parsedTime.Clock()
			year, month, day := referenceDate.Date()
			return time.Date(year, month, day, hour, min, sec, 0, time.Local), nil
		}
		parseErr = err
	}

	return time.Time{}, fmt.Errorf("could not parse time '%s': %v", timeStr, parseErr)
}

func constructRunURL(workflowID uuid.UUID, runID uuid.UUID) string {
	return fmt.Sprintf("https://trickest.io/editor/%s?run=%s", workflowID, runID)
}

func constructNodeURL(runURL, nodeName string) string {
	return fmt.Sprintf("%s&node=%s", runURL, nodeName)
}
