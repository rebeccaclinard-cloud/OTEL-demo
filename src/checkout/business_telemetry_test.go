// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"testing"
	"time"

	pb "github.com/open-telemetry/opentelemetry-demo/src/checkout/genproto/oteldemo"
)

func TestClassifyCheckoutRisk(t *testing.T) {
	tests := []struct {
		name       string
		failed     bool
		retryCount int
		duration   time.Duration
		want       checkoutRisk
	}{
		{name: "healthy", duration: 500 * time.Millisecond, want: checkoutRisk{reason: "none"}},
		{name: "slow", duration: checkoutRiskThreshold + time.Millisecond, want: checkoutRisk{atRisk: true, reason: "slow"}},
		{name: "threshold is not slow", duration: checkoutRiskThreshold, want: checkoutRisk{reason: "none"}},
		{name: "retry takes priority over slow", retryCount: 1, duration: 3 * time.Second, want: checkoutRisk{atRisk: true, reason: "retry"}},
		{name: "failure takes priority", failed: true, retryCount: 1, duration: 3 * time.Second, want: checkoutRisk{atRisk: true, reason: "failure"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := classifyCheckoutRisk(test.failed, test.retryCount, test.duration); got != test.want {
				t.Fatalf("classifyCheckoutRisk() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestMoneyAmountUSD(t *testing.T) {
	value := &pb.Money{Units: 249, Nanos: 990_000_000}
	if got, want := moneyAmountUSD(value), 249.99; got != want {
		t.Fatalf("moneyAmountUSD() = %v, want %v", got, want)
	}
}
