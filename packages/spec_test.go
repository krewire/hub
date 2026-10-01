package packages

import "testing"

func TestParseSpec(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		wantName string
		wantRaw  string
		wantErr  bool
	}{
		{"bare name", "twcss", "twcss", "", false},
		{"pinned version", "twcss@1.2.3", "twcss", "1.2.3", false},
		{"v-prefixed version", "twcss@v1.2.3", "twcss", "v1.2.3", false},
		{"latest", "twcss@latest", "twcss", "latest", false},
		{"go module path", "github.com/foo/bar@v1.2.3", "github.com/foo/bar", "v1.2.3", false},
		{"scoped npm", "@scope/pkg", "@scope/pkg", "", false},
		{"scoped npm with version", "@scope/pkg@1.0.0", "@scope/pkg", "1.0.0", false},
		{"empty", "", "", "", true},
		{"whitespace only", "   ", "", "", true},
		{"missing version", "twcss@", "", "", true},
		{"invalid version", "twcss@not-a-version", "", "", true},
		{"scoped without slash", "@scope", "", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			spec, err := ParseSpec(c.raw)
			if c.wantErr {
				if err == nil {
					t.Fatalf("ParseSpec(%q) error = nil, want error", c.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseSpec(%q) error = %v", c.raw, err)
			}
			if spec.Name != c.wantName {
				t.Errorf("Name = %q, want %q", spec.Name, c.wantName)
			}
			if spec.RawVer != c.wantRaw {
				t.Errorf("RawVer = %q, want %q", spec.RawVer, c.wantRaw)
			}
			if spec.Raw != c.raw {
				t.Errorf("Raw = %q, want %q", spec.Raw, c.raw)
			}
		})
	}
}

func TestSpecEffectiveVersion(t *testing.T) {
	cases := []struct{ raw, want string }{
		{"twcss", "latest"},
		{"twcss@latest", "latest"},
		{"twcss@1.2.3", "1.2.3"},
	}
	for _, c := range cases {
		spec, err := ParseSpec(c.raw)
		if err != nil {
			t.Fatalf("ParseSpec(%q) error = %v", c.raw, err)
		}
		if got := spec.EffectiveVersion(); got != c.want {
			t.Errorf("Spec(%q).EffectiveVersion() = %q, want %q", c.raw, got, c.want)
		}
	}
}

func TestSpecString(t *testing.T) {
	cases := []struct{ raw, want string }{
		{"twcss", "twcss@latest"},
		{"twcss@latest", "twcss@latest"},
		{"twcss@1.2.3", "twcss@1.2.3"},
	}
	for _, c := range cases {
		spec, err := ParseSpec(c.raw)
		if err != nil {
			t.Fatalf("ParseSpec(%q) error = %v", c.raw, err)
		}
		if got := spec.String(); got != c.want {
			t.Errorf("Spec(%q).String() = %q, want %q", c.raw, got, c.want)
		}
	}
}

func TestSpecIsLatest(t *testing.T) {
	latest := []string{"twcss", "twcss@latest"}
	pinned := []string{"twcss@1.2.3", "twcss@v2.0.0"}
	for _, raw := range latest {
		spec, err := ParseSpec(raw)
		if err != nil {
			t.Fatalf("ParseSpec(%q) error = %v", raw, err)
		}
		if !spec.IsLatest() {
			t.Errorf("Spec(%q).IsLatest() = false, want true", raw)
		}
	}
	for _, raw := range pinned {
		spec, err := ParseSpec(raw)
		if err != nil {
			t.Fatalf("ParseSpec(%q) error = %v", raw, err)
		}
		if spec.IsLatest() {
			t.Errorf("Spec(%q).IsLatest() = true, want false", raw)
		}
	}
}

func TestSpecSatisfies(t *testing.T) {
	required := mustVersion(t, "1.2.0")
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{"latest satisfies anything", "twcss@latest", true},
		{"unversioned satisfies anything", "twcss", true},
		{"newer patch satisfies", "twcss@1.2.9", true},
		{"older minor does not satisfy", "twcss@1.1.0", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			spec, err := ParseSpec(c.raw)
			if err != nil {
				t.Fatalf("ParseSpec(%q) error = %v", c.raw, err)
			}
			if got := spec.Satisfies(required); got != c.want {
				t.Errorf("Spec(%q).Satisfies(%s) = %v, want %v", c.raw, required, got, c.want)
			}
		})
	}
}
