package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"playlistconverter/internal/conversions"
)

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func reply(w http.ResponseWriter, r *http.Request, status int, data any, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	var e *apiError
	if code != "" {
		e = &apiError{Code: code, Message: code}
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"success": code == "", "data": data, "error": e, "meta": map[string]any{"requestId": w.Header().Get("X-Request-ID")}})
}
func problem(w http.ResponseWriter, r *http.Request, err error) {
	var e *conversions.Error
	if errors.As(err, &e) {
		reply(w, r, e.Status, nil, e.Code)
		return
	}
	reply(w, r, 500, nil, "internal_error")
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if r.Header.Get("Content-Type") != "application/json" {
		reply(w, r, 415, nil, "json_required")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32768)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		reply(w, r, 400, nil, "invalid_request")
		return false
	}
	if e := d.Decode(new(any)); e != io.EOF {
		reply(w, r, 400, nil, "invalid_request")
		return false
	}
	return true
}
