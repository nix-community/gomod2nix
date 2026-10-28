package generate

import (
	"reflect"
	"testing"

	schema "github.com/nix-community/gomod2nix/internal/schema"
)

func TestGitOrigin(t *testing.T) {
	dl := &goModDownload{
		Path:    "git.example.com/group/apis/go",
		Version: "v1.4.1",
		Origin: &goModOrigin{
			VCS:    "git",
			URL:    "https://git.example.com/group/apis.git",
			Subdir: "go",
			Hash:   "8e06925039022810dcf059d63cfafab75b7efe21",
			Ref:    "refs/tags/go/v1.4.1",
		},
	}

	got := gitOrigin(dl)
	want := &schema.GitOrigin{
		URL:    "https://git.example.com/group/apis.git",
		Ref:    "refs/tags/go/v1.4.1",
		Rev:    "8e06925039022810dcf059d63cfafab75b7efe21",
		Subdir: "go",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("gitOrigin = %+v, want %+v", got, want)
	}
}

func TestGitOriginMissing(t *testing.T) {
	for _, origin := range []*goModOrigin{
		nil,
		{VCS: "hg", URL: "https://hg.example.com/repo", Hash: "abc"},
		{VCS: "git", URL: "https://git.example.com/repo.git"},
	} {
		dl := &goModDownload{Path: "git.example.com/repo", Version: "v1.0.0", Origin: origin}
		if got := gitOrigin(dl); got != nil {
			t.Errorf("gitOrigin(%+v) = %+v, want nil", origin, got)
		}
	}
}
