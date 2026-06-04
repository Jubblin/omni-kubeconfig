package version

import "testing"

func TestParseDefaultVersion(t *testing.T) {
	v, err := Parse()
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if v.Major != 0 || v.Minor != 1 || v.Patch != 0 {
		t.Fatalf("got %s, want 0.1.0", v)
	}
}

func TestParseTolerantGitDescribe(t *testing.T) {
	old := Version
	defer func() { Version = old }()

	Version = "0.1.0-3-gabc1234"
	if _, err := ParseTolerant(); err != nil {
		t.Fatalf("ParseTolerant: %v", err)
	}
}

func TestOmniTag(t *testing.T) {
	if got := OmniTag(); got != "v0.1.0" {
		t.Fatalf("OmniTag() = %q, want v0.1.0", got)
	}
}

func TestStringWithMetadata(t *testing.T) {
	oldVersion, oldCommit, oldDate := Version, Commit, Date
	defer func() {
		Version, Commit, Date = oldVersion, oldCommit, oldDate
	}()

	Version = "1.2.3"
	Commit = "abc1234"
	Date = "2026-06-04T12:00:00Z"

	want := "1.2.3+abc1234 (built 2026-06-04T12:00:00Z)"
	if got := String(); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
