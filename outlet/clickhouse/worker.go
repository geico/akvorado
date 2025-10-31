// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strconv"
	"time"

	"github.com/ClickHouse/ch-go"
	"github.com/cenkalti/backoff/v4"
	"golang.org/x/sync/errgroup"

	"akvorado/common/reporter"
	"akvorado/common/schema"
)

// Worker represents a worker sending to ClickHouse. It is synchronous (no
// goroutines) and most functions are bound to a context.
type Worker interface {
	FinalizeAndSend(context.Context) WorkerStatus
	Flush(context.Context)
}

// WorkerStatus tells if a worker is overloaded or not.
type WorkerStatus int

const (
	// WorkerStatusOK tells the worker is operating in the correct range of efficiency.
	WorkerStatusOK WorkerStatus = iota
	// WorkerStatusOverloaded tells the worker has too much work and more worker would help.
	WorkerStatusOverloaded
	// WorkerStatusUnderloaded tells the worker do not have enough work.
	WorkerStatusUnderloaded
)

// realWorker is a working implementation of Worker.
type realWorker struct {
	c      *realComponent
	bf     *schema.FlowMessage
	last   time.Time
	logger reporter.Logger

	// Per-destination writers (not full workers, just write handlers)
	destWriters []*destinationWriter
}

// destinationWriter handles writes to a single ClickHouse destination
type destinationWriter struct {
	name          string
	conn          *ch.Client
	servers       []string
	options       ch.Options
	asyncSettings []ch.Setting
	config        Configuration
	maxRetries    int
}

// NewWorker creates a new worker to push data to ClickHouse.
func (c *realComponent) NewWorker(i int, bf *schema.FlowMessage) Worker {
	w := realWorker{
		c:           c,
		bf:          bf,
		logger:      c.r.With().Int("worker", i).Logger(),
		destWriters: make([]*destinationWriter, 0, len(c.destinations)),
	}

	// Create a destination writer for each destination
	for _, dest := range c.destinations {
		opts, servers := dest.db.ChGoOptions()
		maxRetries := dest.db.MaxRetries()

		dw := &destinationWriter{
			name:       dest.name,
			servers:    servers,
			options:    opts,
			config:     dest.config,
			maxRetries: maxRetries,
			asyncSettings: []ch.Setting{
				{
					Key:       "async_insert",
					Value:     "1",
					Important: true,
				},
				{
					Key:       "wait_for_async_insert",
					Value:     "1",
					Important: true,
				},
				{
					Key:   "async_insert_busy_timeout_max_ms",
					Value: strconv.FormatUint(uint64(dest.config.MaximumWaitTime.Milliseconds()), 10),
				},
			},
		}
		w.destWriters = append(w.destWriters, dw)
	}

	return &w
}

// FinalizeAndSend sends data to ClickHouse after finalizing if we have a full
// batch or exceeded the maximum wait time. See
// https://clickhouse.com/docs/best-practices/selecting-an-insert-strategy for
// tips on the insert strategy. Notably, we switch to async insert when the
// batch size is too small.
func (w *realWorker) FinalizeAndSend(ctx context.Context) WorkerStatus {
	w.bf.Finalize()
	now := time.Now()
	batchSize := w.bf.FlowCount()
	waitTime := now.Sub(w.last)
	primaryConfig := w.c.primaryConfig()
	if batchSize >= int(primaryConfig.MaximumBatchSize) || waitTime >= primaryConfig.MaximumWaitTime {
		// Record wait time since last send
		if !w.last.IsZero() {
			waitTime := now.Sub(w.last)
			w.c.metrics.waitTime.Observe(waitTime.Seconds())
		}
		w.Flush(ctx)
		w.last = time.Now()
		if uint(batchSize) >= primaryConfig.MaximumBatchSize {
			w.c.metrics.overloaded.Inc()
			return WorkerStatusOverloaded
		} else if uint(batchSize) <= primaryConfig.MaximumBatchSize/minimumBatchSizeDivider {
			w.c.metrics.underloaded.Inc()
			return WorkerStatusUnderloaded
		}
	}
	return WorkerStatusOK
}

// Flush sends remaining data to ClickHouse without an additional condition. It
// should be called before shutting down to flush remaining data. Otherwise,
// FinalizeAndSend() should be used instead.
func (w *realWorker) Flush(ctx context.Context) {
	if w.bf.FlowCount() == 0 {
		return
	}

	// Write to all destinations in parallel
	// NOTE: We use a plain errgroup (not WithContext) so that failures in one
	// destination don't cancel the context for other destinations. Each destination
	// has independent retry limits and should fail independently.
	g := new(errgroup.Group)

	for _, dw := range w.destWriters {
		dw := dw // Capture for goroutine
		g.Go(func() error {
			return w.flushSingleDestination(ctx, dw)
		})
	}

	// Wait for all destinations to complete
	// We don't return the error because we want to clear the batch regardless
	if err := g.Wait(); err != nil {
		w.logger.Err(err).Msg("one or more destinations failed")
	}

	// Clear batch after all destinations have been attempted
	w.bf.Clear()
}

// flushSingleDestination sends data to a single ClickHouse destination with retry logic
func (w *realWorker) flushSingleDestination(ctx context.Context, dw *destinationWriter) error {
	// Async mode if have not a big batch size
	var settings []ch.Setting
	if uint(w.bf.FlowCount()) <= dw.config.MaximumBatchSize/minimumBatchSizeDivider {
		settings = dw.asyncSettings
	}

	// Retry with backoff and max attempts
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 0
	b.MaxInterval = 30 * time.Second
	b.InitialInterval = 20 * time.Millisecond

	attempts := 0
	maxAttempts := dw.maxRetries
	if maxAttempts == 0 {
		maxAttempts = -1 // Infinite retries
	}

	return backoff.Retry(func() error {
		attempts++

		// Check if we've exceeded max retries
		if maxAttempts > 0 && attempts > maxAttempts {
			err := fmt.Errorf("max retries (%d) exceeded for destination %q", maxAttempts, dw.name)
			w.logger.Err(err).
				Str("destination", dw.name).
				Int("attempts", attempts).
				Msg("giving up on destination")
			w.c.metrics.retriesExceeded.WithLabelValues(dw.name).Inc()
			return backoff.Permanent(err) // Stop retrying
		}

		// Connect or reconnect if connection is broken
		if err := w.connectDestination(ctx, dw); err != nil {
			w.logger.Err(err).
				Str("destination", dw.name).
				Int("attempt", attempts).
				Msg("cannot connect to ClickHouse")
			w.c.metrics.errors.WithLabelValues(dw.name, "connect").Inc()
			return err
		}

		// Send to ClickHouse in flows_XXXXX_raw
		start := time.Now()
		if err := dw.conn.Do(ctx, ch.Query{
			Body:     w.bf.ClickHouseProtoInput().Into(fmt.Sprintf("flows_%s_raw", w.c.d.Schema.ClickHouseHash())),
			Input:    w.bf.ClickHouseProtoInput(),
			Settings: settings,
		}); err != nil {
			w.logger.Err(err).
				Str("destination", dw.name).
				Int("flows", w.bf.FlowCount()).
				Int("attempt", attempts).
				Msg("cannot send batch to ClickHouse")
			w.c.metrics.errors.WithLabelValues(dw.name, "send").Inc()

			// Close connection on error
			if dw.conn != nil {
				dw.conn.Close()
				dw.conn = nil
			}
			return err
		}

		// Success - record metrics
		pushDuration := time.Since(start)
		w.c.metrics.insertTime.WithLabelValues(dw.name).Observe(pushDuration.Seconds())
		w.c.metrics.flows.WithLabelValues(dw.name).Observe(float64(w.bf.FlowCount()))

		return nil
	}, backoff.WithContext(b, ctx))
}

// connectDestination establishes or reestablishes the connection to a ClickHouse destination.
func (w *realWorker) connectDestination(ctx context.Context, dw *destinationWriter) error {
	// If connection exists and is healthy, reuse it
	if dw.conn != nil {
		if err := dw.conn.Ping(ctx); err == nil {
			return nil
		}
		// Connection is unhealthy, close it
		dw.conn.Close()
		dw.conn = nil
	}

	// Try each server until one connects successfully
	var lastErr error
	for _, idx := range rand.Perm(len(dw.servers)) {
		dw.options.Address = dw.servers[idx]
		conn, err := ch.Dial(ctx, dw.options)
		if err != nil {
			w.logger.Err(err).
				Str("destination", dw.name).
				Str("server", dw.options.Address).
				Msg("failed to connect to ClickHouse server")
			lastErr = err
			continue
		}

		// Test the connection
		if err := conn.Ping(ctx); err != nil {
			w.logger.Err(err).
				Str("destination", dw.name).
				Str("server", dw.options.Address).
				Msg("ClickHouse server ping failed")
			conn.Close()
			conn = nil
			lastErr = err
			continue
		}

		// Success
		dw.conn = conn
		w.logger.Info().
			Str("destination", dw.name).
			Str("server", dw.options.Address).
			Msg("connected to ClickHouse server")
		return nil
	}

	return lastErr
}
