package httpx

import (
	"encoding/json"
	"net/http"
)

type Envelope struct {
	Data  any       `json:"data,omitempty"`
	Error *APIError `json:"error,omitempty"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Envelope{Data: data})
}

func Error(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Envelope{Error: &APIError{Code: code, Message: message}})
}

func Method(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	return false
}

func DecodeJSON(r *http.Request, out any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(out)
}

func NotImplemented(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		Error(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", name+" endpoint skeleton is registered but not implemented")
	}
}
