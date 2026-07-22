// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace/noop"
)

func TestCheckoutRegressionConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  checkoutRegressionConfig
		wantErr bool
	}{
		{name: "valid", config: checkoutRegressionConfig{slowRatePercent: 30, failureRatePercent: 10, delayMin: time.Second, delayMax: 2 * time.Second}},
		{name: "rates exceed one hundred", config: checkoutRegressionConfig{slowRatePercent: 91, failureRatePercent: 10}, wantErr: true},
		{name: "negative rate", config: checkoutRegressionConfig{slowRatePercent: -1}, wantErr: true},
		{name: "reversed delay range", config: checkoutRegressionConfig{delayMin: 2 * time.Second, delayMax: time.Second}, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if gotErr := test.config.validate() != nil; gotErr != test.wantErr {
				t.Fatalf("validate() error = %v, want error %v", gotErr, test.wantErr)
			}
		})
	}
}

func TestCheckoutRegressionApply(t *testing.T) {
	span := noop.NewTracerProvider().Tracer("test").Start
	tests := []struct {
		name      string
		enabled   bool
		roll      float64
		wantError error
		wantDelay time.Duration
	}{
		{name: "disabled", roll: 0},
		{name: "failure", enabled: true, roll: 0.05, wantError: errSimulatedCheckoutFailure},
		{name: "slow", enabled: true, roll: 0.20, wantDelay: defaultDelayMin},
		{name: "unaffected", enabled: true, roll: 0.80},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var observedDelay time.Duration
			regression := &checkoutRegression{
				config: checkoutRegressionConfig{
					enabled:            test.enabled,
					slowRatePercent:    defaultSlowRatePercent,
					failureRatePercent: defaultFailureRatePercent,
					delayMin:           defaultDelayMin,
					delayMax:           defaultDelayMin,
				},
				randomFloat: func() float64 { return test.roll },
				randomInt63: func(int64) int64 { return 0 },
				wait: func(_ context.Context, delay time.Duration) error {
					observedDelay = delay
					return nil
				},
			}
			ctx, currentSpan := span(context.Background(), "checkout")
			gotError := regression.apply(ctx, currentSpan)
			if !errors.Is(gotError, test.wantError) {
				t.Fatalf("apply() error = %v, want %v", gotError, test.wantError)
			}
			if observedDelay != test.wantDelay {
				t.Fatalf("apply() delay = %v, want %v", observedDelay, test.wantDelay)
			}
		})
	}
}
