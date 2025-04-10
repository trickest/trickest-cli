package stats

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/trickest/trickest-cli/pkg/trickest"
)

type TaskGroupStats struct {
	Count                   int               `json:"count"`
	Status                  SubJobStatus      `json:"status"`
	MinDuration             TaskDuration      `json:"min_duration,omitempty"`
	MaxDuration             TaskDuration      `json:"max_duration,omitempty"`
	Median                  trickest.Duration `json:"median,omitempty"`
	MedianAbsoluteDeviation trickest.Duration `json:"median_absolute_deviation,omitempty"`
	Outliers                []TaskDuration    `json:"outliers,omitempty"`
}

type TaskDuration struct {
	TaskIndex int
	Duration  trickest.Duration
}

type SubJobStatus struct {
	Pending   int `json:"pending"`
	Running   int `json:"running"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
	Stopping  int `json:"stopping"`
	Stopped   int `json:"stopped"`
}

func CalculateTaskGroupStats(sj trickest.SubJob) TaskGroupStats {
	stats := TaskGroupStats{
		Count: len(sj.Children),
		MinDuration: TaskDuration{
			TaskIndex: -1,
			Duration:  trickest.Duration{Duration: time.Duration(math.MaxInt64)},
		},
		MaxDuration: TaskDuration{
			TaskIndex: -1,
			Duration:  trickest.Duration{Duration: time.Duration(math.MinInt64)},
		},
	}

	var taskDurations []TaskDuration
	for _, child := range sj.Children {
		switch child.Status {
		case "PENDING":
			stats.Status.Pending++
		case "RUNNING":
			stats.Status.Running++
		case "SUCCEEDED":
			stats.Status.Succeeded++
		case "FAILED":
			stats.Status.Failed++
		case "STOPPING":
			stats.Status.Stopping++
		case "STOPPED":
			stats.Status.Stopped++
		}

		if child.StartedDate.IsZero() {
			continue
		}

		taskDuration := TaskDuration{
			TaskIndex: child.TaskIndex,
		}

		if child.FinishedDate.IsZero() {
			taskDuration.Duration = trickest.Duration{Duration: time.Since(child.StartedDate)}
		} else {
			taskDuration.Duration = trickest.Duration{Duration: child.FinishedDate.Sub(child.StartedDate)}
		}

		taskDurations = append(taskDurations, taskDuration)

		if taskDuration.Duration.Duration > stats.MaxDuration.Duration.Duration {
			stats.MaxDuration = taskDuration
		}
		if taskDuration.Duration.Duration < stats.MinDuration.Duration.Duration {
			stats.MinDuration = taskDuration
		}
	}

	if len(taskDurations) >= 2 {
		sort.Slice(taskDurations, func(i, j int) bool {
			return taskDurations[i].Duration.Duration < taskDurations[j].Duration.Duration
		})

		medianIndex := len(taskDurations) / 2
		stats.Median = taskDurations[medianIndex].Duration

		// Calculate Median Absolute Deviation (MAD)
		var absoluteDeviations []trickest.Duration
		for _, d := range taskDurations {
			diff := d.Duration.Duration - stats.Median.Duration
			if diff < 0 {
				diff = -diff
			}
			absoluteDeviations = append(absoluteDeviations, trickest.Duration{Duration: diff})
		}
		sort.Slice(absoluteDeviations, func(i, j int) bool {
			return absoluteDeviations[i].Duration < absoluteDeviations[j].Duration
		})
		stats.MedianAbsoluteDeviation = absoluteDeviations[len(absoluteDeviations)/2]

		// Use absolute threshold for outlier detection
		// A task is considered an outlier if it's more than threshold away from the median
		threshold := 15 * time.Minute

		for _, d := range taskDurations {
			diff := d.Duration.Duration - stats.Median.Duration
			if diff < 0 {
				diff = -diff
			}
			if diff > threshold {
				stats.Outliers = append(stats.Outliers, d)
			}
		}
	}

	return stats
}

// HasInterestingStats returns true if the node's stats are worth displaying
func HasInterestingStats(nodeName string) bool {
	unInterestingNodeNames := map[string]bool{
		"batch-output":   true,
		"string-to-file": true,
	}

	parts := strings.Split(nodeName, "-")
	if len(parts) < 2 {
		return true
	}
	baseNodeName := strings.Join(parts[:len(parts)-1], "-")
	return !unInterestingNodeNames[baseNodeName]
}
