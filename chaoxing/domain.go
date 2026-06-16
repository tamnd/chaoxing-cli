package chaoxing

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

func init() { kit.Register(Domain{}) }

// Domain is the Chaoxing kit driver.
type Domain struct{}

// Info describes the scheme and identity for the chaoxing driver.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "chaoxing",
		Hosts:  []string{Host, "mooc1.chaoxing.com", "xuexi.chaoxing.com"},
		Identity: kit.Identity{
			Binary: "cx",
			Short:  "Browse Chaoxing university courses",
			Long: `cx reads public Chaoxing (超星) course data over HTTPS.

The public course catalog is accessible without login. Individual courses
behind authentication return exit 5.

Quick start:
  cx search python              search courses about Python
  cx course 123456              course details by ID`,
			Site: Host,
			Repo: "https://github.com/tamnd/chaoxing-cli",
		},
	}
}

// Register installs the client factory and the two operations onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{
		Name:    "search",
		Group:   "courses",
		Summary: "Search public Chaoxing courses",
		Args:    []kit.Arg{{Name: "keyword", Help: "search keyword"}},
	}, searchOp)

	kit.Handle(app, kit.OpMeta{
		Name:     "course",
		Group:    "courses",
		Single:   true,
		Resolver: true,
		URIType:  "course",
		Summary:  "Fetch course details by ID",
		Args:     []kit.Arg{{Name: "id", Help: "Chaoxing course ID"}},
	}, courseOp)
}

func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := NewClient(DefaultConfig())
	if cfg.UserAgent != "" {
		c.cfg.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.cfg.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.cfg.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.cfg.Timeout = cfg.Timeout
		c.http.Timeout = cfg.Timeout
	}
	return c, nil
}

// --- input structs ---

type searchInput struct {
	Keyword string  `kit:"arg" help:"search keyword"`
	Limit   int     `kit:"flag,inherit" help:"max results"`
	Page    int     `kit:"flag" name:"page" help:"result page number"`
	Client  *Client `kit:"inject"`
}

type courseInput struct {
	ID     string  `kit:"arg" help:"Chaoxing course ID"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func searchOp(ctx context.Context, in searchInput, emit func(*CourseResult) error) error {
	results, err := in.Client.Search(ctx, in.Keyword, SearchOptions{
		Limit: in.Limit,
		Page:  in.Page,
	})
	if err != nil {
		return mapErr(err)
	}
	for i := range results {
		if err := emit(&results[i]); err != nil {
			return err
		}
	}
	return nil
}

func courseOp(ctx context.Context, in courseInput, emit func(*Course) error) error {
	c, err := in.Client.GetCourse(ctx, in.ID)
	if err != nil {
		return mapErr(err)
	}
	return emit(c)
}

// --- Resolver ---

// Classify turns a Chaoxing course URL or bare numeric ID into (type, id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	input = strings.TrimSpace(input)
	if u, err2 := url.Parse(input); err2 == nil &&
		(u.Scheme == "http" || u.Scheme == "https") &&
		strings.Contains(u.Host, "chaoxing.com") {
		if cid := u.Query().Get("courseId"); cid != "" {
			return "course", cid, nil
		}
	}
	if _, err2 := strconv.Atoi(input); err2 == nil && input != "" {
		return "course", input, nil
	}
	return "", "", errs.Usage("unrecognized Chaoxing reference: %q", input)
}

// Locate returns the canonical URL for (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "course":
		return BaseURL + "/mooc-ans/open-course/detail?courseId=" + id, nil
	}
	return "", errs.Usage("chaoxing has no resource type %q", uriType)
}

// mapErr converts library errors to kit error kinds.
func mapErr(err error) error {
	switch {
	case err == ErrBlocked:
		return errs.RateLimited("%s", err.Error())
	case err == ErrNotFound:
		return errs.NotFound("%s", err.Error())
	}
	return err
}
