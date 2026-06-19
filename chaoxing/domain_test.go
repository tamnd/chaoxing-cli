package chaoxing

import (
	"testing"

	"github.com/tamnd/any-cli/kit"
)

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "chaoxing" {
		t.Errorf("Scheme = %q, want chaoxing", info.Scheme)
	}
	if len(info.Hosts) == 0 {
		t.Error("Hosts is empty")
	}
	if info.Identity.Binary != "cx" {
		t.Errorf("Identity.Binary = %q, want cx", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		in      string
		wantTyp string
		wantID  string
		wantErr bool
	}{
		{"123456", "course", "123456", false},
		{"https://mooc1.chaoxing.com/mooc-ans/open-course/detail?courseId=789", "course", "789", false},
		{"", "", "", true},
		{"not-a-number", "", "", true},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("Classify(%q): want error, got nil", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("Classify(%q): unexpected error %v", tc.in, err)
			continue
		}
		if typ != tc.wantTyp || id != tc.wantID {
			t.Errorf("Classify(%q) = (%q, %q), want (%q, %q)",
				tc.in, typ, id, tc.wantTyp, tc.wantID)
		}
	}
}

func TestLocate(t *testing.T) {
	got, err := Domain{}.Locate("course", "12345")
	if err != nil {
		t.Fatal(err)
	}
	want := BaseURL + "/mooc-ans/open-course/detail?courseId=12345"
	if got != want {
		t.Errorf("Locate = %q, want %q", got, want)
	}

	_, err = Domain{}.Locate("unknown", "123")
	if err == nil {
		t.Error("expected error for unknown type")
	}
}

func TestDomainRegistered(t *testing.T) {
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := h.Domain("chaoxing"); !ok {
		t.Fatal("chaoxing domain not registered")
	}
}
