package main

import (
	"context"
	"time"

	"github.com/onflow/flow-go/integration/benchmark"
)

type RawTPSRecord struct {
	Timestamp     time.Time
	OffsetSeconds float64

	InputTPS    float64
	OutputTPS   float64
	TimedoutTPS float64
	ErrorTPS    float64

	InflightTxs int
}

type Status string

const (
	StatusUnknown Status = ""
	StatusSuccess Status = "SUCCESS"
	StatusFailure Status = "FAILURE"
)

// BenchmarkResults is used for uploading data to BigQuery.
type BenchmarkResults struct {
	StartTime       time.Time
	StopTime        time.Time
	DurationSeconds float64

	Status Status

	RawTPS []RawTPSRecord
}

type tpsRecorder struct {
	BenchmarkResults

	lastStats benchmark.WorkerStats
	lastTs    time.Time

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

func NewTPSRecorder(
	ctx context.Context,
	workerStatsTracker *benchmark.WorkerStatsTracker,
) *tpsRecorder {
	ctx, cancel := context.WithCancel(ctx)

	r := &tpsRecorder{
		BenchmarkResults: BenchmarkResults{
			StartTime: time.Now(),
		},
		done:   make(chan struct{}),
		ctx:    ctx,
		cancel: cancel,
	}

	go func() {
		t := time.NewTicker(adjustInterval)
		defer t.Stop()

		defer close(r.done)

		for {
			select {
			case nowTs := <-t.C:
				r.record(nowTs, workerStatsTracker.GetStats())
			case <-ctx.Done():
				return
			}
		}
	}()

	return r
}

func (r *tpsRecorder) Stop() {
	r.cancel()
	<-r.done
	r.StopTime = time.Now()
	r.DurationSeconds = r.StopTime.Sub(r.StartTime).Seconds()
	if r.Status == StatusUnknown {
		r.Status = StatusSuccess
	}
}

func (r *tpsRecorder) SetStatus(status Status) {
	r.Status = status
}

func (r *tpsRecorder) record(nowTs time.Time, stats benchmark.WorkerStats) {
	if !r.lastTs.IsZero() {
		timeDiff := nowTs.Sub(r.lastTs).Seconds()

		r.RawTPS = append(
			r.RawTPS,
			RawTPSRecord{
				Timestamp:     nowTs,
				OffsetSeconds: nowTs.Sub(r.StartTime).Seconds(),

				InputTPS:    float64(stats.TxsSent-r.lastStats.TxsSent) / timeDiff,
				OutputTPS:   float64(stats.TxsExecuted-r.lastStats.TxsExecuted) / timeDiff,
				TimedoutTPS: float64(stats.TxsTimedout-r.lastStats.TxsTimedout) / timeDiff,

				// TODO(rbtz): add error stats.
				ErrorTPS: 0,

				InflightTxs: stats.TxsSent - stats.TxsExecuted,
			})
	}

	r.lastStats = stats
	r.lastTs = nowTs
}
