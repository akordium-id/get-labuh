package docker

import (
	stdtesting "testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateHostPathSecurity(t *stdtesting.T) {
	testCases := []struct {
		name    string
		path    string
		wantErr bool
	}{
		// Valid paths
		{name: "valid opt path", path: "/opt/data/uploads", wantErr: false},
		{name: "valid var subfolder", path: "/var/myapp/storage", wantErr: false},
		{name: "valid home path", path: "/home/deploy/data", wantErr: false},
		{name: "valid srv path", path: "/srv/web/uploads", wantErr: false},

		// Relative paths
		{name: "empty path", path: "", wantErr: true},
		{name: "relative path", path: "relative/path", wantErr: true},
		{name: "dot path", path: ".", wantErr: true},
		{name: "dot slash path", path: "./data", wantErr: true},

		// Blacklisted root and system folders
		{name: "system root", path: "/", wantErr: true},
		{name: "etc directory", path: "/etc", wantErr: true},
		{name: "etc sensitive file", path: "/etc/shadow", wantErr: true},
		{name: "var run socket", path: "/var/run", wantErr: true},
		{name: "docker socket", path: "/var/run/docker.sock", wantErr: true},
		{name: "proc filesystem", path: "/proc", wantErr: true},
		{name: "proc subpath", path: "/proc/cpuinfo", wantErr: true},
		{name: "sys filesystem", path: "/sys", wantErr: true},
		{name: "dev filesystem", path: "/dev", wantErr: true},
		{name: "dev null", path: "/dev/null", wantErr: true},
		{name: "boot directory", path: "/boot", wantErr: true},
		{name: "root user home", path: "/root", wantErr: true},
		{name: "bin directory", path: "/bin", wantErr: true},
		{name: "sbin directory", path: "/sbin", wantErr: true},
		{name: "usr directory", path: "/usr", wantErr: true},
		{name: "lib directory", path: "/lib", wantErr: true},
		{name: "lib64 directory", path: "/lib64", wantErr: true},

		// Traversal attacks
		{name: "traversal to etc", path: "/var/myapp/../../etc/passwd", wantErr: true},
		{name: "traversal to docker socket", path: "/opt/uploads/../../var/run/docker.sock", wantErr: true},
		{name: "traversal to root", path: "/home/user/../../", wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *stdtesting.T) {
			err := ValidateHostPathSecurity(tc.path)
			if tc.wantErr {
				assert.Error(t, err, "expected error for path: %s", tc.path)
			} else {
				assert.NoError(t, err, "expected no error for path: %s", tc.path)
			}
		})
	}
}
