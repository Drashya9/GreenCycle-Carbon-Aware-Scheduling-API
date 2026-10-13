package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/repository"
)

func (s *Server) handleListAppliances(w http.ResponseWriter, r *http.Request) {
	appliances, err := s.applianceRepo.FindAll(r.Context())
	if err != nil {
		writeInternalError(w, "list appliances", err)
		return
	}
	if appliances == nil {
		appliances = []models.Appliance{}
	}
	writeJSON(w, http.StatusOK, appliances)
}

func (s *Server) handleCreateAppliance(w http.ResponseWriter, r *http.Request) {
	var a models.Appliance
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	a.Name = strings.TrimSpace(a.Name)
	if a.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if utf8.RuneCountInString(a.Name) > maxNameLength {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("name must be at most %d characters", maxNameLength))
		return
	}
	a.Protected = false // only seeded rows are protected; a client can't claim it
	if !(a.DurationHours > 0 && a.DurationHours <= maxDurationHours) {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("durationHours must be greater than 0 and at most %d", maxDurationHours))
		return
	}

	if err := s.applianceRepo.Create(r.Context(), &a); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			writeError(w, http.StatusConflict, fmt.Sprintf("appliance %q already exists", a.Name))
			return
		}
		writeInternalError(w, "create appliance", err)
		return
	}
	writeJSON(w, http.StatusCreated, a)
}

func (s *Server) handleDeleteAppliance(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid appliance id")
		return
	}

	if err := s.applianceRepo.Delete(r.Context(), id); err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			writeError(w, http.StatusNotFound, "appliance not found")
		case errors.Is(err, repository.ErrProtected):
			writeError(w, http.StatusForbidden, "the seeded demo appliances can't be deleted")
		case errors.Is(err, repository.ErrHasPendingRuns):
			writeError(w, http.StatusConflict, "this appliance has scheduled runs waiting to fire; cancel them first")
		default:
			writeInternalError(w, "delete appliance", err)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
