package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestFlags(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expectedError string
	}{
		{
			name:          "should fail because src is not set",
			args:          []string{"-password", "test", "-zone-name", "test"},
			expectedError: "src is required",
		},
		{
			name:          "should fail because endpoint is set to empty string",
			args:          []string{"-src", "testdata", "-endpoint", "", "-password", "test", "-zone-name", "test"},
			expectedError: "endpoint is required",
		},
		{
			name:          "should fail because password is set to empty string",
			args:          []string{"-src", "testdata", "-password", "", "-zone-name", "test"},
			expectedError: "password is required",
		},
		{
			name:          "should fail because zone-name is set to empty string",
			args:          []string{"-src", "testdata", "-password", "test", "-zone-name", ""},
			expectedError: "zone-name is required",
		},
		{
			name:          "should fail because folder testdata does not exists",
			args:          []string{"-src", "testdata", "-password", "test", "-zone-name", "test"},
			expectedError: "is not a directory or does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf safeBuffer

			err := run(&buf, tt.args)
			if err != nil && tt.expectedError == "" {
				t.Fatalf("error %q", err)
			}
			if err == nil && tt.expectedError != "" {
				t.Fatalf("expected error %q, got nil", tt.expectedError)
			}
			if err != nil && !strings.Contains(err.Error(), tt.expectedError) {
				t.Fatalf("expected error %q, got %q", tt.expectedError, err)
			}
		})
	}
}

func TestUploadScenario(t *testing.T) {
	player := requestsPlayer{
		cache:   make(map[string]CachedResponse),
		records: len(os.Getenv("TEST_RECORD")) > 0,
	}
	srv := player.init()
	defer srv.Close()

	pswd := "pswd"
	zoneName := "pang0-wronged-vexingly"

	if !player.records {
		player.load()
	} else {
		pswd = os.Getenv("BUNNY_PASSWORD")
	}

	var buf safeBuffer

	dir1Files := []string{
		"file1",
		"nd1/file2",
		"nd2/nd21/file3",
	}
	// upload files
	player.next()
	dir1 := createDirectory(t, dir1Files)
	err := run(&buf, []string{"-src", dir1, "-endpoint", srv.URL, "-password", pswd, "-zone-name", zoneName})
	if err != nil {
		t.Fatal(err)
	}
	s1 := buf.String()
	for _, p := range dir1Files {
		expected := fmt.Sprintf("+ %s\n", p)
		if !strings.Contains(s1, expected) {
			t.Fatalf("expected to see %q in %q", expected, s1)
		}
	}

	// remove files
	player.next()
	buf = safeBuffer{}
	dir2 := createDirectory(t, []string{})
	err = run(&buf, []string{"-src", dir2, "-endpoint", srv.URL, "-password", pswd, "-zone-name", zoneName})
	if err != nil {
		t.Fatal(err)
	}
	s2 := buf.String()
	for _, p := range dir1Files {
		expected := fmt.Sprintf("- %s\n", p)
		if !strings.Contains(s2, expected) {
			t.Fatalf("expected to see %q in %q", expected, s2)
		}
	}

	// state should remain empty
	player.next()
	buf = safeBuffer{}
	err = run(&buf, []string{"-src", dir2, "-endpoint", srv.URL, "-password", pswd, "-zone-name", zoneName})
	if err != nil {
		t.Fatal(err)
	}
	s3 := buf.String()
	if s3 != "" {
		t.Fatalf("expected empty state, got %q", s3)
	}

	if player.records {
		player.flush()
	}
}

type safeBuffer struct {
	buf bytes.Buffer
	sync.Mutex
}

func (sb *safeBuffer) Write(data []byte) (int, error) {
	sb.Lock()
	defer sb.Unlock()
	return sb.buf.Write(data)
}

func (sb *safeBuffer) String() string {
	sb.Lock()
	defer sb.Unlock()
	return sb.buf.String()
}

func createDirectory(t *testing.T, filePaths []string) string {
	t.Helper()

	dirPath := t.TempDir()

	for _, filePath := range filePaths {
		err := os.MkdirAll(filepath.Dir(filepath.Join(dirPath, filePath)), os.ModePerm)
		if err != nil {
			t.Fatal(err)
		}

		err = os.WriteFile(filepath.Join(dirPath, filePath), []byte(filePath), os.ModePerm)
		if err != nil {
			t.Fatal(err)
		}
	}

	return dirPath
}

type CachedResponse struct {
	Body    []byte
	Code    int
	Headers map[string]string
}

type requestsPlayer struct {
	cache map[string]CachedResponse
	sync.RWMutex

	nextNum int
	records bool
}

func (p *requestsPlayer) next() {
	p.nextNum++
}

func (p *requestsPlayer) record(w http.ResponseWriter, r *http.Request) {
	p.Lock()
	defer p.Unlock()

	key := fmt.Sprintf("%d:%s:%s", p.nextNum, r.Method, r.URL.Path)

	bunnyStorageEndpoint := os.Getenv("BUNNY_STORAGE_ENDPOINT") + r.URL.Path
	req, err := http.NewRequest(r.Method, bunnyStorageEndpoint, r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	req.Header = r.Header

	c := &http.Client{}
	res, err := c.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer res.Body.Close()

	b, err := io.ReadAll(res.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	p.cache[key] = CachedResponse{
		Body:    b,
		Code:    res.StatusCode,
		Headers: make(map[string]string),
	}

	for k := range res.Header {
		vv := res.Header.Get(k)
		p.cache[key].Headers[k] = vv
		w.Header().Add(k, vv)
	}
	w.WriteHeader(res.StatusCode)
	w.Write(b)
}

func (p *requestsPlayer) replay(w http.ResponseWriter, r *http.Request) {
	key := fmt.Sprintf("%d:%s:%s", p.nextNum, r.Method, r.URL.Path)

	resp, ok := p.cache[key]
	if !ok {
		http.Error(w, "not found cached key", http.StatusInternalServerError)
		return
	}

	for k, v := range resp.Headers {
		w.Header().Add(k, v)
	}
	w.WriteHeader(resp.Code)
	w.Write(resp.Body)
}

func (p *requestsPlayer) flush() error {
	b, err := json.MarshalIndent(p.cache, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile("api_fixtures.json", b, os.ModePerm)
}

func (p *requestsPlayer) load() error {
	b, err := os.ReadFile("api_fixtures.json")
	if err != nil {
		return err
	}

	return json.Unmarshal(b, &p.cache)
}

func (p *requestsPlayer) init() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p.records {
			p.record(w, r)
			return
		}

		p.replay(w, r)
	}))
}
