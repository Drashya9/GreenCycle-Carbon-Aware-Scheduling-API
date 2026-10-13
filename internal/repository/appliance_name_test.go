package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
)

// TestApplianceNamesAreUniqueIgnoringCaseAndSpaces runs against a real Postgres (skipped
// without one; CI provides it) with migration 0003 applied.
func TestApplianceNamesAreUniqueIgnoringCaseAndSpaces(t *testing.T) {
	dbx := openTestDB(t)
	repo := NewApplianceRepository(dbx)

	name := fmt.Sprintf("Case Test %d", time.Now().UnixNano())
	first := &models.Appliance{Name: name, DurationHours: 1}
	if err := repo.Create(context.Background(), first); err != nil {
		t.Fatalf("create: %v", err)
	}
	defer repo.Delete(context.Background(), first.ID)

	for _, variant := range []string{strings.ToLower(name), strings.ToUpper(name), " " + name + " "} {
		err := repo.Create(context.Background(), &models.Appliance{Name: variant, DurationHours: 3})
		if !errors.Is(err, ErrConflict) {
			t.Errorf("%q: expected ErrConflict, got %v", variant, err)
		}
	}

	found, err := repo.FindByNameIgnoreCase(context.Background(), "  "+strings.ToLower(name)+"  ")
	if err != nil || found.ID != first.ID {
		t.Errorf("expected the padded, lowercased lookup to find id %d, got %+v, %v", first.ID, found, err)
	}
}
