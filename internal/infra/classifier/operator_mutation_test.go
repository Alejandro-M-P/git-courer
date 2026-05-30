package classifier

import "testing"

// TestDetectOperatorMutation_boundary tests boundary operator change.
// Changing >=20 to >20 adjusts boundary logic — that's a fix.
func TestDetectOperatorMutation_boundary(t *testing.T) {
	diff := `-    if x >= 20 {
+    if x > 20 {
`
	commitType, confidence := detectOperatorMutation(diff)
	if commitType != "fix" {
		t.Errorf("expected fix for boundary operator change, got %q", commitType)
	}
	if confidence != 0.95 {
		t.Errorf("expected confidence 0.95, got %f", confidence)
	}
}

// TestDetectOperatorMutation_logic tests logical operator change.
func TestDetectOperatorMutation_logic(t *testing.T) {
	diff := `-    return a && b
+    return a || b
`
	commitType, confidence := detectOperatorMutation(diff)
	if commitType != "fix" {
		t.Errorf("expected fix for logical operator change, got %q", commitType)
	}
	if confidence != 0.95 {
		t.Errorf("expected confidence 0.95, got %f", confidence)
	}
}

// TestDetectOperatorMutation_equality tests equality operator change.
func TestDetectOperatorMutation_equality(t *testing.T) {
	diff := `-    if status == "ok" {
+    if status != "ok" {
`
	commitType, confidence := detectOperatorMutation(diff)
	if commitType != "fix" {
		t.Errorf("expected fix for equality operator change, got %q", commitType)
	}
	if confidence != 0.95 {
		t.Errorf("expected confidence 0.95, got %f", confidence)
	}
}

// TestDetectOperatorMutation_negation tests negation operator addition.
func TestDetectOperatorMutation_negation(t *testing.T) {
	diff := `-    if ready {
+    if !ready {
`
	commitType, confidence := detectOperatorMutation(diff)
	if commitType != "fix" {
		t.Errorf("expected fix for negation operator change, got %q", commitType)
	}
	if confidence != 0.95 {
		t.Errorf("expected confidence 0.95, got %f", confidence)
	}
}

// TestDetectOperatorMutation_no_change tests identical operators on both sides.
func TestDetectOperatorMutation_no_change(t *testing.T) {
	diff := `-    return a + b
+    return a + b
`
	commitType, confidence := detectOperatorMutation(diff)
	if commitType != "" {
		t.Errorf("expected empty when operators don't change, got %q", commitType)
	}
	if confidence != 0.0 {
		t.Errorf("expected confidence 0.0 for no change, got %f", confidence)
	}
}

// TestDetectOperatorMutation_no_operators tests when diff has no operators at all.
func TestDetectOperatorMutation_no_operators(t *testing.T) {
	diff := `-    return "hello"
+    return "world"
`
	commitType, confidence := detectOperatorMutation(diff)
	if commitType != "" {
		t.Errorf("expected empty when no operators present, got %q", commitType)
	}
	if confidence != 0.0 {
		t.Errorf("expected confidence 0.0, got %f", confidence)
	}
}

// TestDetectOperatorMutation_multiple tests detecting multiple operator changes.
func TestDetectOperatorMutation_multiple(t *testing.T) {
	diff := `-    if x > 10 && y < 5 {
+    if x >= 10 || y <= 5 {
`
	commitType, confidence := detectOperatorMutation(diff)
	if commitType != "fix" {
		t.Errorf("expected fix for multiple operator changes, got %q", commitType)
	}
	if confidence != 0.95 {
		t.Errorf("expected confidence 0.95, got %f", confidence)
	}
}

// TestDetectOperatorMutation_empty tests empty diff.
func TestDetectOperatorMutation_empty(t *testing.T) {
	commitType, confidence := detectOperatorMutation("")
	if commitType != "" {
		t.Errorf("expected empty for empty diff, got %q", commitType)
	}
	if confidence != 0.0 {
		t.Errorf("expected confidence 0.0, got %f", confidence)
	}
}

// TestDetectOperatorMutation_only_additions tests diff with only additions.
func TestDetectOperatorMutation_only_additions(t *testing.T) {
	diff := `+    if x > 10 {
+        return true
+    }
`
	commitType, confidence := detectOperatorMutation(diff)
	if commitType != "" {
		t.Errorf("expected empty when only additions, got %q", commitType)
	}
	if confidence != 0.0 {
		t.Errorf("expected confidence 0.0, got %f", confidence)
	}
}

// TestDetectOperatorMutation_complex_expression tests operator in nested expression.
func TestDetectOperatorMutation_complex_expression(t *testing.T) {
	diff := `-    if (a > b) && (c == d) || !e {
+    if (a >= b) || (c != d) && e {
`
	commitType, confidence := detectOperatorMutation(diff)
	if commitType != "fix" {
		t.Errorf("expected fix for complex expression operator change, got %q", commitType)
	}
	if confidence != 0.95 {
		t.Errorf("expected confidence 0.95, got %f", confidence)
	}
}
