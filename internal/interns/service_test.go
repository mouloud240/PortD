package interns

import "testing"

func TestNull(t *testing.T) {
	if null("").Valid || !null("x").Valid {
		t.Fatal("unexpected null conversion")
	}
}
