// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	defaultSlowRatePercent    = 30
	defaultFailureRatePercent = 10
	defaultDelayMin           = 2400 * time.Millisecond
	defaultDelayMax           = 3500 * time.Millisecond
)

var errSimulatedCheckoutFailure = errors.New("simulated payment provider failure")

type checkoutRegressionConfig struct {
	enabled            bool
	slowRatePercent    int
	failureRatePercent int
	delayMin           time.Duration
	delayMax           time.Duration
}

type checkoutRegression struct {
	config      checkoutRegressionConfig
	randomFloat func() float64
	randomInt63 func(int64) int64
	wait        func(context.Context, time.Duration) error
}

func checkoutRegressionFromEnvironment() (*checkoutRegression, error) {
	enabled, err := boolEnvironment("CHECKOUT_REGRESSION_ENABLED", false)
	if err != nil {
		return nil, err
	}

	slowRate, err := intEnvironment("CHECKOUT_SLOW_RATE_PERCENT", defaultSlowRatePercent)
	if err != nil {
		return nil, err
	}
	failureRate, err := intEnvironment("CHECKOUT_FAILURE_RATE_PERCENT", defaultFailureRatePercent)
	if err != nil {
		return nil, err
	}
	delayMinMs, err := intEnvironment("CHECKOUT_DELAY_MIN_MS", int(defaultDelayMin/time.Millisecond))
	if err != nil {
		return nil, err
	}
	delayMaxMs, err := intEnvironment("CHECKOUT_DELAY_MAX_MS", int(defaultDelayMax/time.Millisecond))
	if err != nil {
		return nil, err
	}

	config := checkoutRegressionConfig{
		enabled:            enabled,
		slowRatePercent:    slowRate,
		failureRatePercent: failureRate,
		delayMin:           time.Duration(delayMinMs) * time.Millisecond,
		delayMax:           time.Duration(delayMaxMs) * time.Millisecond,
	}
	if err := config.validate(); err != nil {
		return nil, err
	}

	return &checkoutRegression{
		config:      config,
		randomFloat: rand.Float64,
		randomInt63: rand.Int63n,
		wait:        waitForRegressionDelay,
	}, nil
}

func (c checkoutRegressionConfig) validate() error {
	if c.slowRatePercent < 0 || c.failureRatePercent < 0 || c.slowRatePercent+c.failureRatePercent > 100 {
		return fmt.Errorf("checkout regression rates must be non-negative and total no more than 100 percent")
	}
	if c.delayMin < 0 || c.delayMax < c.delayMin {
		return fmt.Errorf("checkout regression delay range is invalid")
	}
	return nil
}

func (r *checkoutRegression) apply(ctx context.Context, span trace.Span) error {
	span.SetAttributes(attribute.Bool("demo.checkout.regression.enabled", r.config.enabled))
	if !r.config.enabled {
		return nil
	}

	roll := r.randomFloat() * 100
	if roll < float64(r.config.failureRatePercent) {
		span.SetAttributes(
			attribute.String("demo.checkout.regression.component", "payment-provider"),
			attribute.String("demo.checkout.regression.outcome", "failure"),
		)
		return errSimulatedCheckoutFailure
	}

	if roll >= float64(r.config.failureRatePercent+r.config.slowRatePercent) {
		span.SetAttributes(attribute.String("demo.checkout.regression.outcome", "none"))
		return nil
	}

	delay := r.config.delayMin
	if difference := r.config.delayMax - r.config.delayMin; difference > 0 {
		delay += time.Duration(r.randomInt63(int64(difference) + 1))
	}
	span.SetAttributes(
		attribute.String("demo.checkout.regression.component", "payment-provider"),
		attribute.String("demo.checkout.regression.outcome", "slow"),
		attribute.Int64("demo.checkout.regression.delay_ms", delay.Milliseconds()),
	)
	return r.wait(ctx, delay)
}

func waitForRegressionDelay(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func boolEnvironment(name string, fallback bool) (bool, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", name, err)
	}
	return parsed, nil
}

func intEnvironment(name string, fallback int) (int, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	return parsed, nil
}
