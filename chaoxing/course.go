package chaoxing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// GetCourse fetches a single course by its Chaoxing course ID.
func (c *Client) GetCourse(ctx context.Context, id string) (*Course, error) {
	base := c.cfg.BaseURL
	if base == "" {
		base = BaseURL
	}
	u := fmt.Sprintf("%s/api/course/detail?courseId=%s", base, url.QueryEscape(id))
	body, err := c.Get(ctx, u)
	if err != nil {
		return nil, err
	}
	return parseCourse(id, body, base)
}

type courseDetailResp struct {
	Result bool      `json:"result"`
	Msg    string    `json:"msg"`
	Data   rawCourse `json:"data"`
}

type rawCourseDetail struct {
	CourseID    int      `json:"courseId"`
	Name        string   `json:"name"`
	Teacher     string   `json:"teacherfactor"`
	Schoolname  string   `json:"schoolname"`
	MemberCount int      `json:"memberCount"`
	ImageURL    string   `json:"imageUrl"`
	StartTime   string   `json:"startTime"`
	EndTime     string   `json:"endTime"`
	Description string   `json:"description"`
	Chapters    []string `json:"chapters"`
}

var chapterRE = regexp.MustCompile(`<[^>]+class="[^"]*chapter[^"]*"[^>]*>\s*([^<]+)`)

func parseCourse(id string, body []byte, baseURL string) (*Course, error) {
	if baseURL == "" {
		baseURL = BaseURL
	}
	courseURL := baseURL + "/mooc-ans/open-course/detail?courseId=" + id

	// Try JSON first
	var resp struct {
		Result bool            `json:"result"`
		Msg    string          `json:"msg"`
		Data   rawCourseDetail `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err == nil && resp.Result {
		d := resp.Data
		return &Course{
			ID:          id,
			Name:        d.Name,
			Teacher:     d.Teacher,
			Institution: d.Schoolname,
			Members:     d.MemberCount,
			Description: d.Description,
			StartDate:   d.StartTime,
			EndDate:     d.EndTime,
			ImageURL:    d.ImageURL,
			Chapters:    d.Chapters,
			URL:         courseURL,
		}, nil
	}

	// JSON but result=false — check for auth error
	var errResp struct {
		Result bool   `json:"result"`
		Msg    string `json:"msg"`
	}
	if jerr := json.Unmarshal(body, &errResp); jerr == nil && !errResp.Result {
		if strings.Contains(errResp.Msg, "登录") || strings.Contains(errResp.Msg, "login") {
			return nil, ErrBlocked
		}
		if strings.Contains(errResp.Msg, "不存在") || strings.Contains(errResp.Msg, "not found") {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("api error: %s", errResp.Msg)
	}

	// HTML fallback
	return parseCourseHTML(id, body, courseURL), nil
}

var (
	courseTitleRE  = regexp.MustCompile(`(?i)<h1[^>]*class="[^"]*course-title[^"]*"[^>]*>([^<]+)</h1>`)
	courseH1RE     = regexp.MustCompile(`<h1[^>]*>([^<]+)</h1>`)
	teacherNameRE  = regexp.MustCompile(`(?i)class="[^"]*teacher-name[^"]*"[^>]*>([^<]+)<`)
	schoolNameRE   = regexp.MustCompile(`(?i)class="[^"]*school-name[^"]*"[^>]*>([^<]+)<`)
	memberCountRE  = regexp.MustCompile(`(?i)class="[^"]*member-count[^"]*"[^>]*>([^<]+)<`)
	courseDescRE   = regexp.MustCompile(`(?s)class="[^"]*course-desc[^"]*"[^>]*>(.+?)</`)
	tagStripRE     = regexp.MustCompile(`<[^>]+>`)
)

func parseCourseHTML(id string, body []byte, courseURL string) *Course {
	s := string(body)
	c := &Course{ID: id, URL: courseURL}

	if m := courseTitleRE.FindStringSubmatch(s); m != nil {
		c.Name = strings.TrimSpace(m[1])
	} else if m := courseH1RE.FindStringSubmatch(s); m != nil {
		c.Name = strings.TrimSpace(m[1])
	}
	if m := teacherNameRE.FindStringSubmatch(s); m != nil {
		c.Teacher = strings.TrimSpace(m[1])
	}
	if m := schoolNameRE.FindStringSubmatch(s); m != nil {
		c.Institution = strings.TrimSpace(m[1])
	}
	if m := courseDescRE.FindStringSubmatch(s); m != nil {
		c.Description = strings.TrimSpace(tagStripRE.ReplaceAllString(m[1], ""))
	}
	for _, m := range chapterRE.FindAllStringSubmatch(s, -1) {
		name := strings.TrimSpace(m[1])
		if name != "" {
			c.Chapters = append(c.Chapters, name)
		}
	}
	return c
}
