package api

import (
	"encoding/json"
	"net/http"

	"github.com/legoser/gsm2mqtt/internal/tariff"
)

func (s *Server) handleTariffStatus(w http.ResponseWriter, r *http.Request) {
	modemID := r.URL.Query().Get("modem_id")
	st, err := s.manager.GetTariffStatus(modemID)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(st)
}

func (s *Server) handleTariffConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ModemID          string   `json:"modem_id"`
		SMSLimit         *int     `json:"sms_limit"`
		CallMinutesLimit *float64 `json:"call_minutes_limit"`
		ResetDayOfMonth  *int     `json:"reset_day_of_month"`
		MinBalanceAlert  *float64 `json:"min_balance_alert"`
		BalanceUSSD      *string  `json:"balance_ussd"`
		OperatorPreset   *string  `json:"operator_preset"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request json")
		return
	}

	cfg := tariff.Config{}
	if req.SMSLimit != nil {
		cfg.SMSLimit = *req.SMSLimit
	}
	if req.CallMinutesLimit != nil {
		cfg.CallMinutesLimit = *req.CallMinutesLimit
	}
	if req.ResetDayOfMonth != nil {
		cfg.ResetDayOfMonth = *req.ResetDayOfMonth
	}
	if req.MinBalanceAlert != nil {
		cfg.MinBalanceAlert = *req.MinBalanceAlert
	}
	if req.BalanceUSSD != nil {
		cfg.BalanceUSSD = *req.BalanceUSSD
	}
	if req.OperatorPreset != nil {
		cfg.OperatorPreset = *req.OperatorPreset
	}

	if err := s.manager.UpdateTariffConfig(req.ModemID, cfg); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (s *Server) handleTariffReset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ModemID string `json:"modem_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if err := s.manager.ResetTariffQuotas(req.ModemID); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
