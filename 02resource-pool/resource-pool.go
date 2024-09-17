package main

import (
	"encoding/json"

	"github.com/google/uuid"

	"bufio"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"
)

type Device struct {
	DevID    string `json:"devid"`
	UUID     string `json:"uuid"`
	HostPort string `json:"hostport"`
}

var (
	deviceLookUpTable []Device
	devIDToUUIDMap    = make(map[string]string)
)

// Handles the GET /resources request
func getResources(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(deviceLookUpTable); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		log.Println("Error encoding response:", err)
	}
}

// Handles the POST /allocation request
func attachResource(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var device Device
	if err := json.NewDecoder(r.Body).Decode(&device); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		log.Println("Error decoding request body:", err)
		return
	}

	found := false
	for index, dev := range deviceLookUpTable {
		if dev.DevID == device.DevID {
			if dev.HostPort != "" {
				http.Error(w, "Device is already attached. Detach it first.", http.StatusBadRequest)
				return
			}
			if device.HostPort == "" {
				http.Error(w, "HostPort is not given.", http.StatusBadRequest)
				return
			}
			deviceLookUpTable[index].HostPort = device.HostPort // Attach resource (set HostPort)
			found = true
			break
		}
	}

	if !found {
		http.Error(w, "Device not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Handles the DELETE /allocation request
func detachResource(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var device Device
	if err := json.NewDecoder(r.Body).Decode(&device); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		log.Println("Error decoding request body:", err)
		return
	}

	found := false
	for index, dev := range deviceLookUpTable {
		if dev.DevID == device.DevID {
			deviceLookUpTable[index].HostPort = "" // Detach resource
			found = true
			break
		}
	}

	if !found {
		http.Error(w, "Device not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Reads the configuration file and populates the device lookup table
func parseResourceConfig(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		parts := strings.Split(line, ",")
		if len(parts) != 2 {
			return fmt.Errorf("invalid format at line %d: '%s', expected 2 values separated by a comma", lineNum, line)
		}
		device := Device{
			DevID:    parts[0],
			UUID:     uuid.New().String(),
			HostPort: parts[1],
		}

		deviceLookUpTable = append(deviceLookUpTable, device)
		devIDToUUIDMap[device.DevID] = device.UUID
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

// Initializes the server and routes
func startServer() {
	r := mux.NewRouter()
	r.HandleFunc("/resources", getResources).Methods("GET")
	r.HandleFunc("/allocation", attachResource).Methods("POST")
	r.HandleFunc("/allocation", detachResource).Methods("DELETE")
	log.Fatal(http.ListenAndServe(":8000", r))
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: ./resource-pool <resource-config-path>")
		return
	}

	path := os.Args[1]
	if err := parseResourceConfig(path); err != nil {
		log.Fatalf("Error parsing resource configuration file: %v", err)
	}

	startServer()
}
