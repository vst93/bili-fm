//go:build linux && (amd64 || arm64)

package linux

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGPUDevice(t *testing.T) {
	system := func(t *testing.T, devices []string, drivers map[string]string) string {
		root := t.TempDir()
		for _, d := range devices {
			p := filepath.Join(root, "dev", d)
			os.MkdirAll(filepath.Dir(p), 0o755)
			os.WriteFile(p, nil, 0o644)
		}
		for node, driver := range drivers {
			dev := filepath.Join(root, "sys", "class", "drm", node, "device")
			os.MkdirAll(dev, 0o755)
			if err := os.Symlink(filepath.Join("..", "..", "bus", "pci", "drivers", driver), filepath.Join(dev, "driver")); err != nil {
				t.Fatal(err)
			}
		}
		return root
	}
	for _, c := range []struct {
		name    string
		devices []string
		drivers map[string]string
		gallium string
		want    bool
	}{
		{"nothing", nil, nil, "", false},
		{"Intel", []string{"dri/renderD128"}, map[string]string{"renderD128": "i915"}, "", true},
		{"AMD and simpledrm", nil, map[string]string{"renderD128": "simpledrm", "renderD129": "amdgpu"}, "", true},
		{"a virtual machine", []string{"dri/card0"}, map[string]string{"renderD128": "bochs-drm"}, "", false},
		{"NVIDIA", []string{"nvidia0"}, nil, "", true},
		{"WSL", []string{"dxg"}, nil, "", false},
		{"WSL with d3d12", []string{"dxg"}, nil, "d3d12", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := gpuDeviceIn(system(t, c.devices, c.drivers), c.gallium); got != c.want {
				t.Errorf("gpuDevice = %v, want %v", got, c.want)
			}
		})
	}
}
