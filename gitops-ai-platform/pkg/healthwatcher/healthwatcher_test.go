package healthwatcher

import (
	"context"
	"testing"
)

const healthyPodListJSON = `{
  "items": [
    {
      "metadata": {"name": "myapp-abc123"},
      "status": {
        "phase": "Running",
        "containerStatuses": [{"restartCount": 0, "state": {"running": {}}}]
      }
    }
  ]
}`

const crashLoopPodListJSON = `{
  "items": [
    {
      "metadata": {"name": "myapp-def456"},
      "status": {
        "phase": "Running",
        "containerStatuses": [{"restartCount": 5, "state": {"waiting": {"reason": "CrashLoopBackOff"}}}]
      }
    }
  ]
}`

func TestParsePodListJSON_Healthy(t *testing.T) {
	pods, err := parsePodListJSON([]byte(healthyPodListJSON))
	if err != nil {
		t.Fatalf("parsePodListJSON() error = %v", err)
	}
	if len(pods) != 1 {
		t.Fatalf("expected 1 pod, got %d", len(pods))
	}
	if pods[0].Phase != "Running" || pods[0].WaitingReason != "" {
		t.Errorf("unexpected pod status: %+v", pods[0])
	}
}

func TestParsePodListJSON_CrashLoop(t *testing.T) {
	pods, err := parsePodListJSON([]byte(crashLoopPodListJSON))
	if err != nil {
		t.Fatalf("parsePodListJSON() error = %v", err)
	}
	if pods[0].WaitingReason != "CrashLoopBackOff" {
		t.Errorf("WaitingReason = %q, want CrashLoopBackOff", pods[0].WaitingReason)
	}
	if pods[0].RestartCount != 5 {
		t.Errorf("RestartCount = %d, want 5", pods[0].RestartCount)
	}
}

func TestEvaluate_HealthyPods(t *testing.T) {
	pods := []PodStatus{{Name: "a", Phase: "Running", RestartCount: 0}}
	result := Evaluate(pods)
	if !result.Healthy {
		t.Errorf("expected healthy, got unhealthy: %s", result.Reason)
	}
}

func TestEvaluate_CrashLoopIsUnhealthy(t *testing.T) {
	pods := []PodStatus{{Name: "a", Phase: "Running", WaitingReason: "CrashLoopBackOff"}}
	result := Evaluate(pods)
	if result.Healthy {
		t.Fatal("expected unhealthy for CrashLoopBackOff")
	}
	if result.Reason == "" {
		t.Error("expected a non-empty reason")
	}
}

func TestEvaluate_HighRestartCountIsUnhealthy(t *testing.T) {
	pods := []PodStatus{{Name: "a", Phase: "Running", RestartCount: 10}}
	result := Evaluate(pods)
	if result.Healthy {
		t.Fatal("expected unhealthy for high restart count")
	}
}

func TestEvaluate_NoPodsIsUnhealthy(t *testing.T) {
	result := Evaluate(nil)
	if result.Healthy {
		t.Fatal("expected unhealthy when no pods are found")
	}
}

func TestRootCause_FallsBackWithoutGeminiClient(t *testing.T) {
	health := HealthResult{Healthy: false, Reason: "pod x is in CrashLoopBackOff"}
	explanation, fix := RootCause(context.Background(), nil, "myapp", health, "")
	if explanation == "" || fix == "" {
		t.Error("expected non-empty fallback explanation and fix even with no Gemini client")
	}
}
