// Package update finds, verifies and installs new Modbus Studio releases
// published on GitHub.
package update

import (
	"fmt"
	"strconv"
	"strings"
)

// Version is a parsed semantic version (semver 2.0, build metadata ignored).
type Version struct {
	Major, Minor, Patch int
	Pre                 []string // pre-release identifiers, e.g. ["beta", "2"]
}

// ParseVersion accepts "1.2.3", "v1.2.3-beta.1" or "1.2.3+build".
func ParseVersion(s string) (Version, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if i := strings.IndexByte(s, '+'); i >= 0 {
		s = s[:i]
	}
	var v Version
	core := s
	if i := strings.IndexByte(s, '-'); i >= 0 {
		core = s[:i]
		v.Pre = strings.Split(s[i+1:], ".")
		for _, p := range v.Pre {
			if p == "" {
				return Version{}, fmt.Errorf("invalid version %q", s)
			}
		}
	}
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("invalid version %q", s)
	}
	nums := [3]int{}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return Version{}, fmt.Errorf("invalid version %q", s)
		}
		nums[i] = n
	}
	v.Major, v.Minor, v.Patch = nums[0], nums[1], nums[2]
	return v, nil
}

// IsPrerelease reports whether v has pre-release identifiers.
func (v Version) IsPrerelease() bool { return len(v.Pre) > 0 }

func (v Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if len(v.Pre) > 0 {
		s += "-" + strings.Join(v.Pre, ".")
	}
	return s
}

// Compare returns -1, 0 or 1 following semver precedence rules.
func (v Version) Compare(o Version) int {
	for _, d := range [3][2]int{{v.Major, o.Major}, {v.Minor, o.Minor}, {v.Patch, o.Patch}} {
		if d[0] != d[1] {
			return cmp(d[0], d[1])
		}
	}
	// A version without pre-release ranks above one with it.
	switch {
	case len(v.Pre) == 0 && len(o.Pre) == 0:
		return 0
	case len(v.Pre) == 0:
		return 1
	case len(o.Pre) == 0:
		return -1
	}
	for i := 0; i < len(v.Pre) && i < len(o.Pre); i++ {
		a, b := v.Pre[i], o.Pre[i]
		an, aerr := strconv.Atoi(a)
		bn, berr := strconv.Atoi(b)
		switch {
		case aerr == nil && berr == nil:
			if an != bn {
				return cmp(an, bn)
			}
		case aerr == nil: // numeric identifiers rank lower
			return -1
		case berr == nil:
			return 1
		default:
			if a != b {
				return strings.Compare(a, b)
			}
		}
	}
	return cmp(len(v.Pre), len(o.Pre))
}

func cmp(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
