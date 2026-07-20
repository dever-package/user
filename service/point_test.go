package service

import (
	"testing"
	"time"
)

func TestValidatePointChangeAmount(t *testing.T) {
	if err := validatePointChangeAmount(0, "form.amount"); err == nil {
		t.Fatal("zero point amount must be rejected")
	}
	if err := validatePointChangeAmount(maxPointChangeAmount, "form.amount"); err != nil {
		t.Fatalf("maximum point amount should be accepted: %v", err)
	}
	if err := validatePointChangeAmount(maxPointChangeAmount+1, "form.amount"); err == nil {
		t.Fatal("point amount above the limit must be rejected")
	}
}

func TestPointAdjustmentRetryDelayIsBounded(t *testing.T) {
	previous := time.Duration(0)
	for attempt := 0; attempt < pointAdjustmentMaxAttempts; attempt++ {
		delay := pointAdjustmentRetryDelay(attempt)
		if delay < previous {
			t.Fatalf("retry delay decreased at attempt %d: %v < %v", attempt, delay, previous)
		}
		if delay > maxPointAdjustmentRetryDelay {
			t.Fatalf("retry delay exceeded maximum: %v", delay)
		}
		previous = delay
	}
}
