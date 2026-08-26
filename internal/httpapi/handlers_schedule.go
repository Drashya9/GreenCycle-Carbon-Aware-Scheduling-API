package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/repository"
)

type createScheduleRequest struct {
	ApplianceName string `json:"applianceName"`
	Zip           string `json:"zip"`
	WebhookURL    string `json:"webhookUrl,omitempty"`
}

// handleCreateSchedule computes the optimal window for an appliance/zip via the same
// pipeline as GET /calculate, then persists it as a pending scheduled run. The
// background scheduler engine picks it up once its start time arrives.
func (s *Server) handleCreateSchedule(w http.ResponseWriter, r *http.Request) {
	var req createScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	req.ApplianceName = strings.TrimSpace(req.ApplianceName)
	req.Zip = strings.TrimSpace(req.Zip)
	if req.ApplianceName == "" {
		writeError(w, http.StatusBadRequest, "applianceName is required")
		return
	}
	if req.Zip == "" {
		writeError(w, http.StatusBadRequest, "zip is required")
		return
	}

	appliance, err := s.applianceRepo.FindByNameIgnoreCase(r.Context(), req.ApplianceName)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusBadRequest, "appliance not found: '"+req.ApplianceName+"'")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	window, err := s.calculateWindow(r.Context(), appliance.Name, req.Zip)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	run := &models.ScheduledRun{
		ApplianceID:            appliance.ID,
		ApplianceName:          appliance.Name,
		ZipCode:                req.Zip,
		StartTime:              window.StartTime,
		EndTime:                window.EndTime,
		AverageCarbonIntensity: window.AverageCarbonIntensity,
	}
	if req.WebhookURL != "" {
		run.WebhookURL = sql.Null[string]{V: req.WebhookURL, Valid: true}
	}

	if err := s.runRepo.Create(r.Context(), run); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create scheduled run: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, run)
}

func (s *Server) handleListSchedules(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	runs, err := s.runRepo.FindAll(r.Context(), status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list scheduled runs: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, runs)
}

func (s *Server) handleGetSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid schedule id")
		return
	}

	run, err := s.runRepo.FindByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (s *Server) handleCancelSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid schedule id")
		return
	}

	if err := s.runRepo.Cancel(r.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "scheduled run not found or no longer pending")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
