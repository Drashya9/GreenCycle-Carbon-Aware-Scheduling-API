package models

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestNullMarshalsAsPlainValueOrNull(t *testing.T) {
	cases := map[string]struct {
		in   any
		want string
	}{
		"absent string":  {Null[string]{}, `null`},
		"present string": {NewNull("https://example.com/hook"), `"https://example.com/hook"`},
		"absent time":    {Null[time.Time]{}, `null`},
		"present time":   {NewNull(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)), `"2026-01-02T03:04:05Z"`},
	}
	for name, c := range cases {
		got, err := json.Marshal(c.in)
		if err != nil {
			t.Fatalf("%s: marshal: %v", name, err)
		}
		if string(got) != c.want {
			t.Errorf("%s: got %s, want %s", name, got, c.want)
		}
	}
}

func TestNullUnmarshalRoundTrip(t *testing.T) {
	var present Null[string]
	if err := json.Unmarshal([]byte(`"boom"`), &present); err != nil {
		t.Fatalf("unmarshal value: %v", err)
	}
	if !present.Valid || present.V != "boom" {
		t.Errorf("expected valid \"boom\", got %+v", present)
	}

	absent := NewNull("stale")
	if err := json.Unmarshal([]byte(`null`), &absent); err != nil {
		t.Fatalf("unmarshal null: %v", err)
	}
	if absent.Valid {
		t.Errorf("expected null to clear the value, got %+v", absent)
	}
}

// TestScheduledRunJSONHasNoStructShape guards the API contract the dashboard and any client
// relies on: nullable columns serialize as values or null, never {"V":..,"Valid":..}.
func TestScheduledRunJSONHasNoStructShape(t *testing.T) {
	run := ScheduledRun{ID: 1, Status: StatusFailed, LastError: NewNull("connection refused")}
	body, err := json.Marshal(run)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(body)
	for _, want := range []string{`"webhookUrl":null`, `"firedAt":null`, `"lastError":"connection refused"`} {
		if !strings.Contains(s, want) {
			t.Errorf("expected %s in %s", want, s)
		}
	}
	if strings.Contains(s, `"Valid"`) {
		t.Errorf("sql.Null struct shape leaked into JSON: %s", s)
	}
}
