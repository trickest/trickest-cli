package stats

import (
	"math"
	"sort"
	"time"

	"github.com/trickest/trickest-cli/pkg/trickest"
)

type TaskGroupStats struct {
	Count                   int
	Status                  SubJobStatus
	MinDuration             TaskDuration
	MaxDuration             TaskDuration
	Median                  time.Duration
	MedianAbsoluteDeviation time.Duration
	Outliers                []TaskDuration
}

type TaskDuration struct {
	TaskIndex int
	Duration  time.Duration
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
			Duration:  time.Duration(math.MaxInt64),
		},
		MaxDuration: TaskDuration{
			TaskIndex: -1,
			Duration:  time.Duration(math.MinInt64),
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
			taskDuration.Duration = time.Since(child.StartedDate)
		} else {
			taskDuration.Duration = child.FinishedDate.Sub(child.StartedDate)
		}

		taskDurations = append(taskDurations, taskDuration)

		if taskDuration.Duration > stats.MaxDuration.Duration {
			stats.MaxDuration = taskDuration
		}
		if taskDuration.Duration < stats.MinDuration.Duration {
			stats.MinDuration = taskDuration
		}
	}

	if len(taskDurations) >= 2 {
		sort.Slice(taskDurations, func(i, j int) bool {
			return taskDurations[i].Duration < taskDurations[j].Duration
		})

		medianIndex := len(taskDurations) / 2
		stats.Median = taskDurations[medianIndex].Duration

		// Calculate Median Absolute Deviation (MAD)
		var absoluteDeviations []time.Duration
		for _, d := range taskDurations {
			diff := d.Duration - stats.Median
			if diff < 0 {
				diff = -diff
			}
			absoluteDeviations = append(absoluteDeviations, diff)
		}
		sort.Slice(absoluteDeviations, func(i, j int) bool {
			return absoluteDeviations[i] < absoluteDeviations[j]
		})
		stats.MedianAbsoluteDeviation = absoluteDeviations[len(absoluteDeviations)/2]

		// Use absolute threshold for outlier detection
		// A task is considered an outlier if it's more than threshold away from the median
		threshold := 15 * time.Minute

		for _, d := range taskDurations {
			diff := d.Duration - stats.Median
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
