package bunnyclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type BunnyClient struct {
	client                 *http.Client
	primaryStorageEndpoint string
	storageZoneName        string
	storageAPIKey          string
}

const bunnyClientTimeout = 60 * time.Second

type ListObjectResponse struct {
	Checksum      string `json:"Checksum"`
	IsDirectory   bool   `json:"IsDirectory"`
	ObjectName    string `json:"ObjectName"`
	Path          string `json:"Path"`
	CorrectedPath string
}

func New(primaryStorageEndpoint, storageZoneName, storageAPIKey string) *BunnyClient {
	return &BunnyClient{
		client:                 &http.Client{},
		primaryStorageEndpoint: primaryStorageEndpoint,
		storageZoneName:        storageZoneName,
		storageAPIKey:          storageAPIKey,
	}
}

func (bc *BunnyClient) List(ctx context.Context, path string) ([]ListObjectResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, bunnyClientTimeout)
	defer cancel()

	req, err := http.NewRequest(http.MethodGet, bc.primaryStorageEndpoint, nil)
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)
	req.URL.Path = fmt.Sprintf("/%s%s", bc.storageZoneName, path)
	req.Header.Set("AccessKey", bc.storageAPIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := bc.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var objects []ListObjectResponse
	err = json.NewDecoder(resp.Body).Decode(&objects)
	if err != nil {
		return nil, err
	}

	prefix := fmt.Sprintf("/%s/", bc.storageZoneName)
	for i := range objects {
		path := strings.TrimPrefix(objects[i].Path+objects[i].ObjectName, prefix)
		if objects[i].IsDirectory {
			objects[i].CorrectedPath = fmt.Sprintf("/%s/", path)
		} else {
			objects[i].CorrectedPath = path
		}
	}

	return objects, nil
}

func (bc *BunnyClient) Upload(ctx context.Context, path string, data []byte) error {
	ctx, cancel := context.WithTimeout(ctx, bunnyClientTimeout)
	defer cancel()

	req, err := http.NewRequest(http.MethodPut, bc.primaryStorageEndpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}

	req = req.WithContext(ctx)
	req.URL.Path = fmt.Sprintf("/%s/%s", bc.storageZoneName, path)
	req.Header.Set("AccessKey", bc.storageAPIKey)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Accept", "application/json")

	resp, err := bc.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func (bc *BunnyClient) Delete(ctx context.Context, path string) error {
	ctx, cancel := context.WithTimeout(ctx, bunnyClientTimeout)
	defer cancel()

	req, err := http.NewRequest(http.MethodDelete, bc.primaryStorageEndpoint, nil)
	if err != nil {
		return err
	}

	req = req.WithContext(ctx)
	req.URL.Path = fmt.Sprintf("/%s/%s", bc.storageZoneName, path)
	req.Header.Set("AccessKey", bc.storageAPIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := bc.client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
