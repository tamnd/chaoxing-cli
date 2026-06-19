package chaoxing

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestClient(baseURL string) *Client {
	cfg := DefaultConfig()
	cfg.BaseURL = baseURL
	cfg.Rate = 0
	cfg.Retries = 2
	cfg.Timeout = 5 * time.Second
	return NewClient(cfg)
}

func TestGetSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"result":true,"data":{"list":[],"total":0}}`)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	body, err := c.Get(context.Background(), srv.URL+"/test")
	if err != nil {
		t.Fatal(err)
	}
	if len(body) == 0 {
		t.Error("expected non-empty body")
	}
}

func TestGetBlocked403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.Get(context.Background(), srv.URL+"/test")
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("expected ErrBlocked, got %v", err)
	}
}

func TestGetBlockedRedirectToLogin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "https://passport.chaoxing.com/login")
		w.WriteHeader(http.StatusFound)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.Get(context.Background(), srv.URL+"/test")
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("expected ErrBlocked for login redirect, got %v", err)
	}
}

func TestGetBlockedJSONAuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"result":false,"msg":"未登录，请先登录"}`)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.Get(context.Background(), srv.URL+"/test")
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("expected ErrBlocked for JSON auth error, got %v", err)
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = fmt.Fprint(w, `{"result":true,"data":{"list":[]}}`)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	c.cfg.Retries = 5
	body, err := c.Get(context.Background(), srv.URL+"/test")
	if err != nil {
		t.Fatal(err)
	}
	if len(body) == 0 {
		t.Error("expected body after retry")
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
}

func TestSearchSuccess(t *testing.T) {
	respBody := `{
		"result": true,
		"data": {
			"total": 2,
			"list": [
				{"courseId": 1001, "name": "Python Fundamentals", "teacherfactor": "Dr. Li",
				 "schoolname": "Tsinghua University", "memberCount": 50000,
				 "startTime": "2024-02-20", "endTime": "2024-06-30"},
				{"courseId": 1002, "name": "Python Advanced", "teacherfactor": "Prof. Wang",
				 "schoolname": "PKU", "memberCount": 30000,
				 "startTime": "2024-03-01", "endTime": "2024-07-31"}
			]
		}
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, respBody)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	results, err := c.Search(context.Background(), "python", SearchOptions{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("want 2 results, got %d", len(results))
	}
	r := results[0]
	if r.Name != "Python Fundamentals" {
		t.Errorf("Name = %q, want Python Fundamentals", r.Name)
	}
	if r.Teacher != "Dr. Li" {
		t.Errorf("Teacher = %q, want Dr. Li", r.Teacher)
	}
	if r.Institution != "Tsinghua University" {
		t.Errorf("Institution = %q", r.Institution)
	}
	if r.Members != 50000 {
		t.Errorf("Members = %d, want 50000", r.Members)
	}
	if r.ID != "1001" {
		t.Errorf("ID = %q, want 1001", r.ID)
	}
}

func TestSearchBlockedAuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"result":false,"msg":"请登录后访问"}`)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.Search(context.Background(), "test", SearchOptions{})
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("expected ErrBlocked, got %v", err)
	}
}

func TestSearchBlocked403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.Search(context.Background(), "test", SearchOptions{})
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("expected ErrBlocked from Search, got %v", err)
	}
}

func TestCourseSuccess(t *testing.T) {
	respBody := `{
		"result": true,
		"data": {
			"courseId": 5001,
			"name": "Data Structures",
			"teacherfactor": "Prof. Zhang",
			"schoolname": "SJTU",
			"memberCount": 12000,
			"description": "Learn data structures.",
			"startTime": "2024-02-15",
			"endTime": "2024-06-15"
		}
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, respBody)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	course, err := c.GetCourse(context.Background(), "5001")
	if err != nil {
		t.Fatal(err)
	}
	if course.Name != "Data Structures" {
		t.Errorf("Name = %q, want Data Structures", course.Name)
	}
	if course.Teacher != "Prof. Zhang" {
		t.Errorf("Teacher = %q, want Prof. Zhang", course.Teacher)
	}
	if course.Institution != "SJTU" {
		t.Errorf("Institution = %q, want SJTU", course.Institution)
	}
}

func TestCourseNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"result":false,"msg":"课程不存在"}`)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.GetCourse(context.Background(), "9999")
	if err == nil {
		t.Fatal("expected error for not-found course")
	}
}

func TestCourseBlocked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.GetCourse(context.Background(), "123")
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("expected ErrBlocked, got %v", err)
	}
}

func TestIsBlocked(t *testing.T) {
	cases := []struct {
		name   string
		status int
		loc    string
		body   string
		want   bool
	}{
		{"403", 403, "", "", true},
		{"login redirect", 302, "https://passport.chaoxing.com/login", "", true},
		{"json auth error", 200, "", `{"result":false,"msg":"未登录"}`, true},
		{"normal json", 200, "", `{"result":true,"data":{"list":[]}}`, false},
	}
	for _, tc := range cases {
		h := http.Header{}
		if tc.loc != "" {
			h.Set("Location", tc.loc)
		}
		resp := &http.Response{StatusCode: tc.status, Header: h}
		got := isBlocked(resp, []byte(tc.body))
		if got != tc.want {
			t.Errorf("%s: isBlocked = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestParseSearchResults(t *testing.T) {
	body := []byte(`{"result":true,"data":{"total":1,"list":[
		{"courseId":42,"name":"Go Programming","teacherfactor":"Alice","schoolname":"MIT",
		 "memberCount":9999,"startTime":"2024-01-01","endTime":"2024-05-01"}
	]}}`)
	results, err := parseSearchResults(body, "https://test.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("want 1 result, got %d", len(results))
	}
	if results[0].Name != "Go Programming" {
		t.Errorf("Name = %q", results[0].Name)
	}
	if results[0].Members != 9999 {
		t.Errorf("Members = %d, want 9999", results[0].Members)
	}
}

func TestParseCourseJSON(t *testing.T) {
	body := []byte(`{"result":true,"data":{
		"courseId":7777,"name":"OS Fundamentals","teacherfactor":"Bob",
		"schoolname":"MIT","memberCount":8000,"description":"Operating systems.",
		"startTime":"2024-03-01","endTime":"2024-07-01"
	}}`)
	course, err := parseCourse("7777", body, "https://test.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if course.Name != "OS Fundamentals" {
		t.Errorf("Name = %q", course.Name)
	}
	if course.Description != "Operating systems." {
		t.Errorf("Description = %q", course.Description)
	}
}

// Verify that backoff produces increasing durations.
func TestBackoffIncreases(t *testing.T) {
	d1 := backoff(1)
	d2 := backoff(2)
	d3 := backoff(3)
	if d1 >= d2 || d2 >= d3 {
		t.Errorf("backoff not increasing: %v %v %v", d1, d2, d3)
	}
}
