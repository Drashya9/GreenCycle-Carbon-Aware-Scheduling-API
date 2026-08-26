package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/repository"
)

func (s *Server) handleListAppliances(w http.ResponseWriter, r *http.Request) {
	appliances, err := s.applianceRepo.FindAll(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list appliances: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, appliances)
}

func (s *Server) handleCreateAppliance(w http.ResponseWriter, r *http.Request) {
	var a models.Appliance
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if strings.TrimSpace(a.Name) == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if a.DurationHours <= 0 {
		writeError(w, http.StatusBadRequest, "durationHours must be greater than 0")
		return
	}

	if err := s.applianceRepo.Create(r.Context(), &a); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create appliance: "+err.Error())
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
		if errors.Is(err, repository.ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete appliance: "+err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
