// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	pb "github.com/open-telemetry/opentelemetry-demo/src/checkout/genproto/oteldemo"
)

const checkoutRiskThreshold = 2 * time.Second

type checkoutRisk struct {
	atRisk bool
	reason string
}

type checkoutMeasurement struct {
	cartValueUSD float64
	duration     time.Duration
	retryCount   int
	failed       bool
}

type checkoutBusinessMetrics struct {
	transactions  metric.Int64Counter
	revenue       metric.Float64Counter
	revenueAtRisk metric.Float64Counter
	retries       metric.Int64Counter
	duration      metric.Float64Histogram
}

func newCheckoutBusinessMetrics(meter metric.Meter) (*checkoutBusinessMetrics, error) {
	transactions, err := meter.Int64Counter(
		"business.checkout.transactions",
		metric.WithDescription("Number of checkout transaction attempts."),
		metric.WithUnit("{transaction}"),
	)
	if err != nil {
		return nil, fmt.Errorf("create checkout transaction counter: %w", err)
	}

	revenue, err := meter.Float64Counter(
		"business.checkout.revenue",
		metric.WithDescription("Total dollar value of checkout attempts."),
		metric.WithUnit("USD"),
	)
	if err != nil {
		return nil, fmt.Errorf("create checkout revenue counter: %w", err)
	}

	revenueAtRisk, err := meter.Float64Counter(
		"business.checkout.revenue_at_risk",
		metric.WithDescription("Checkout revenue at risk because of failure, retry, or latency over two seconds."),
		metric.WithUnit("USD"),
	)
	if err != nil {
		return nil, fmt.Errorf("create revenue at risk counter: %w", err)
	}

	retries, err := meter.Int64Counter(
		"business.checkout.retries",
		metric.WithDescription("Number of checkout operation retries."),
		metric.WithUnit("{retry}"),
	)
	if err != nil {
		return nil, fmt.Errorf("create checkout retry counter: %w", err)
	}

	duration, err := meter.Float64Histogram(
		"business.checkout.duration",
		metric.WithDescription("End-to-end checkout transaction duration."),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("create checkout duration histogram: %w", err)
	}

	return &checkoutBusinessMetrics{
		transactions:  transactions,
		revenue:       revenue,
		revenueAtRisk: revenueAtRisk,
		retries:       retries,
		duration:      duration,
	}, nil
}

func classifyCheckoutRisk(failed bool, retryCount int, duration time.Duration) checkoutRisk {
	switch {
	case failed:
		return checkoutRisk{atRisk: true, reason: "failure"}
	case retryCount > 0:
		return checkoutRisk{atRisk: true, reason: "retry"}
	case duration > checkoutRiskThreshold:
		return checkoutRisk{atRisk: true, reason: "slow"}
	default:
		return checkoutRisk{reason: "none"}
	}
}

func (m *checkoutBusinessMetrics) recordCheckout(ctx context.Context, span trace.Span, measurement checkoutMeasurement) {
	risk := classifyCheckoutRisk(measurement.failed, measurement.retryCount, measurement.duration)
	outcome := "success"
	if measurement.failed {
		outcome = "failed"
	}

	revenueAtRiskUSD := 0.0
	if risk.atRisk {
		revenueAtRiskUSD = measurement.cartValueUSD
	}

	span.SetAttributes(
		attribute.String("business.transaction.type", "checkout"),
		attribute.String("business.checkout.outcome", outcome),
		attribute.Bool("business.checkout.failed", measurement.failed),
		attribute.Bool("business.checkout.retried", measurement.retryCount > 0),
		attribute.Bool("business.checkout.slow", measurement.duration > checkoutRiskThreshold),
		attribute.Int("business.checkout.retry_count", measurement.retryCount),
		attribute.Float64("business.checkout.duration_ms", float64(measurement.duration.Microseconds())/1000),
		attribute.Float64("business.cart.value_usd", measurement.cartValueUSD),
		attribute.Bool("business.revenue_at_risk", risk.atRisk),
		attribute.Float64("business.revenue_at_risk_usd", revenueAtRiskUSD),
		attribute.String("business.risk.reason", risk.reason),
	)

	attributes := metric.WithAttributes(
		attribute.String("business.checkout.outcome", outcome),
		attribute.String("business.risk.reason", risk.reason),
		attribute.Bool("business.revenue_at_risk", risk.atRisk),
	)

	m.transactions.Add(ctx, 1, attributes)
	m.duration.Record(ctx, measurement.duration.Seconds(), attributes)
	if measurement.cartValueUSD > 0 {
		m.revenue.Add(ctx, measurement.cartValueUSD, attributes)
	}
	if measurement.retryCount > 0 {
		m.retries.Add(ctx, int64(measurement.retryCount), attributes)
	}
	if risk.atRisk && measurement.cartValueUSD > 0 {
		m.revenueAtRisk.Add(ctx, measurement.cartValueUSD, attributes)
	}
}

func moneyAmountUSD(value *pb.Money) float64 {
	return float64(value.GetUnits()) + float64(value.GetNanos())/1_000_000_000
}
