package alertexpr

import (
	"testing"
)

func TestSimpleComparison(t *testing.T) {
	e := NewEvaluator(map[string]float64{"cpu": 95})
	result, err := e.Eval("cpu > 90")
	if err != nil {
		t.Fatal(err)
	}
	if !result {
		t.Error("expected true, got false")
	}

	result, err = e.Eval("cpu > 95")
	if err != nil {
		t.Fatal(err)
	}
	if result {
		t.Error("expected false, got true")
	}
}

func TestComparisonOps(t *testing.T) {
	e := NewEvaluator(map[string]float64{"cpu": 90})
	tests := []struct {
		expr string
		want bool
	}{
		{"cpu >= 90", true},
		{"cpu >= 91", false},
		{"cpu < 91", true},
		{"cpu < 90", false},
		{"cpu <= 90", true},
		{"cpu <= 89", false},
		{"cpu == 90", true},
		{"cpu == 91", false},
		{"cpu != 91", true},
		{"cpu != 90", false},
		{"cpu = 90", true},
	}
	for _, tc := range tests {
		got, err := e.Eval(tc.expr)
		if err != nil {
			t.Errorf("expr %q: %v", tc.expr, err)
			continue
		}
		if got != tc.want {
			t.Errorf("expr %q: got %v, want %v", tc.expr, got, tc.want)
		}
	}
}

func TestLogicalAND(t *testing.T) {
	e := NewEvaluator(map[string]float64{"cpu": 95, "mem": 85})
	result, err := e.Eval("cpu > 90 AND mem > 80")
	if err != nil {
		t.Fatal(err)
	}
	if !result {
		t.Error("expected true, got false")
	}

	result, err = e.Eval("cpu > 90 AND mem > 90")
	if err != nil {
		t.Fatal(err)
	}
	if result {
		t.Error("expected false, got true")
	}
}

func TestLogicalOR(t *testing.T) {
	e := NewEvaluator(map[string]float64{"cpu": 95, "mem": 50})
	result, err := e.Eval("cpu > 90 OR mem > 80")
	if err != nil {
		t.Fatal(err)
	}
	if !result {
		t.Error("expected true, got false")
	}

	result, err = e.Eval("cpu < 50 OR mem > 80")
	if err != nil {
		t.Fatal(err)
	}
	if result {
		t.Error("expected false, got true")
	}
}

func TestParentheses(t *testing.T) {
	e := NewEvaluator(map[string]float64{"cpu": 60, "mem": 85, "disk": 96})
	// (cpu > 90 AND mem > 80) OR disk > 95
	result, err := e.Eval("(cpu > 90 AND mem > 80) OR disk > 95")
	if err != nil {
		t.Fatal(err)
	}
	if !result {
		t.Error("expected true, got false")
	}

	// (cpu > 90 OR mem > 80) AND disk > 95
	result, err = e.Eval("(cpu > 90 OR mem > 80) AND disk > 95")
	if err != nil {
		t.Fatal(err)
	}
	if !result {
		t.Error("expected true, got false")
	}

	// (cpu > 90 OR mem > 80) AND disk > 98
	result, err = e.Eval("(cpu > 90 OR mem > 80) AND disk > 98")
	if err != nil {
		t.Fatal(err)
	}
	if result {
		t.Error("expected false, got true")
	}
}

func TestLiteralValues(t *testing.T) {
	e := NewEvaluator(map[string]float64{})
	result, err := e.Eval("100 > 90")
	if err != nil {
		t.Fatal(err)
	}
	if !result {
		t.Error("expected true, got false")
	}
}

func TestCombinedExpression(t *testing.T) {
	metrics := map[string]float64{
		"cpu":         92,
		"mem":         88,
		"disk":        80,
		"connections": 500,
		"qps":         12000,
	}
	e := NewEvaluator(metrics)
	expr := "(cpu > 90 AND mem > 85) OR (connections > 1000 AND qps > 5000)"
	result, err := e.Eval(expr)
	if err != nil {
		t.Fatal(err)
	}
	if !result {
		t.Error("expected true, got false — cpu+mem should trigger")
	}
}
