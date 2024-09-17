package inter

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type FalconInterface struct {
	getResourceEndpoint string
	reconfigEndpoint    string
}

type DevicePair struct {
	DevID    string
	HostPort string
}

func NewDevInterface(getResourceEndpoint string, reconfigEndpoint string) *FalconInterface {
	return &FalconInterface{
		getResourceEndpoint: getResourceEndpoint,
		reconfigEndpoint:    reconfigEndpoint,
	}
}

func (fi *FalconInterface) GetAllResource() ([]DevicePair, error) {
	body, err := fi.sendRequest(http.MethodGet, fi.getResourceEndpoint, nil)
	if err != nil {
		return nil, err
	}

	// Parses the result
	var result []map[string]string
	if err := json.Unmarshal([]byte(string(body)), &result); err != nil { // Parse []byte to the go struct pointer
		return nil, fmt.Errorf("error unmarshalling JSON: %v", err)
	}

	// Returns all devices
	var devices []DevicePair
	for _, res := range result {
		devices = append(devices, DevicePair{
			DevID:    res["devid"],
			HostPort: res["hostport"],
		})
	}
	return devices, nil
}

func (fi *FalconInterface) Assign(hostPort string, devid string) (bool, error) {
	param := fmt.Sprintf(`{"hostport" : "%s", "devid" : "%s"}`, hostPort, devid)
	_, err := fi.sendRequest(http.MethodPost, fi.reconfigEndpoint, strings.NewReader(param))
	if err != nil {
		return false, err
	}
	return true, nil
}

func (fi *FalconInterface) Unassign(devid string) (bool, error) {
	param := fmt.Sprintf(`{"devid" : "%s"}`, devid)
	_, err := fi.sendRequest(http.MethodDelete, fi.reconfigEndpoint, strings.NewReader(param))
	if err != nil {
		return false, err
	}
	return true, nil
}

func (fi *FalconInterface) sendRequest(method, url string, payload io.Reader) ([]byte, error) {
	client := &http.Client{}

	req, err := http.NewRequest(method, url, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Performs the request
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer res.Body.Close()

	// Reads and return the response body
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP request error: %s", string(body))
	}

	return body, nil
}
