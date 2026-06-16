package chaoxing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// SearchOptions controls a Search call.
type SearchOptions struct {
	Limit int
	Page  int
}

// Search searches Chaoxing's public course catalog for keyword.
func (c *Client) Search(ctx context.Context, keyword string, opts SearchOptions) ([]CourseResult, error) {
	if opts.Limit <= 0 {
		opts.Limit = 20
	}
	if opts.Page <= 0 {
		opts.Page = 1
	}
	base := c.cfg.BaseURL
	if base == "" {
		base = BaseURL
	}
	u := fmt.Sprintf("%s/api/search/course?keyword=%s&pageNum=%d&pageSize=%d",
		base, url.QueryEscape(keyword), opts.Page, opts.Limit)
	body, err := c.Get(ctx, u)
	if err != nil {
		return nil, err
	}
	return parseSearchResults(body, base)
}

type searchResp struct {
	Result bool   `json:"result"`
	Msg    string `json:"msg"`
	Data   struct {
		List  []rawCourse `json:"list"`
		Total int         `json:"total"`
	} `json:"data"`
}

type rawCourse struct {
	CourseID      int    `json:"courseId"`
	Name          string `json:"name"`
	Teacherfactor string `json:"teacherfactor"`
	Schoolname    string `json:"schoolname"`
	MemberCount   int    `json:"memberCount"`
	ImageURL      string `json:"imageUrl"`
	StartTime     string `json:"startTime"`
	EndTime       string `json:"endTime"`
}

func parseSearchResults(body []byte, baseURL string) ([]CourseResult, error) {
	var resp searchResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse search: %w", err)
	}
	if !resp.Result {
		msg := resp.Msg
		if strings.Contains(msg, "登录") || strings.Contains(msg, "login") {
			return nil, ErrBlocked
		}
		return nil, fmt.Errorf("api error: %s", msg)
	}
	if baseURL == "" {
		baseURL = BaseURL
	}
	out := make([]CourseResult, 0, len(resp.Data.List))
	for _, r := range resp.Data.List {
		id := strconv.Itoa(r.CourseID)
		out = append(out, CourseResult{
			ID:          id,
			Name:        r.Name,
			Teacher:     r.Teacherfactor,
			Institution: r.Schoolname,
			Members:     r.MemberCount,
			StartDate:   r.StartTime,
			EndDate:     r.EndTime,
			ImageURL:    r.ImageURL,
			URL:         baseURL + "/mooc-ans/open-course/detail?courseId=" + id,
		})
	}
	return out, nil
}
