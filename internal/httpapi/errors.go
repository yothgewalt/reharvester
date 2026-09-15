package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
)

// writeErrors answers a validation failure: 400 with {"errors": {field:
// message}}. Used wherever a request carries user-editable fields — settings
// and harvest options — so the client can point at the field that failed
// rather than parse a sentence.
func writeErrors(w http.ResponseWriter, errs map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	if err := json.NewEncoder(w).Encode(map[string]map[string]string{"errors": errs}); err != nil {
		log.Printf("api: encode: %v", err)
	}
}
