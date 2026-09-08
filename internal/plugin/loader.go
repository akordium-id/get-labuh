package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"plugin"
)

type Loader struct {
	pluginsDir string
}

func NewLoader(pluginsDir string) *Loader {
	return &Loader{pluginsDir: pluginsDir}
}

func (l *Loader) Load() ([]Plugin, error) {
	var loaded []Plugin

	if _, err := os.Stat(l.pluginsDir); os.IsNotExist(err) {
		return loaded, nil
	}

	entries, err := os.ReadDir(l.pluginsDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := filepath.Ext(entry.Name())
		if ext != ".so" {
			continue
		}

		path := filepath.Join(l.pluginsDir, entry.Name())
		p, err := plugin.Open(path)
		if err != nil {
			fmt.Printf("Warning: failed to load plugin %s: %v\n", entry.Name(), err)
			continue
		}

		sym, err := p.Lookup("NewPlugin")
		if err != nil {
			fmt.Printf("Warning: plugin %s missing NewPlugin symbol: %v\n", entry.Name(), err)
			continue
		}

		newPlugin, ok := sym.(func() Plugin)
		if !ok {
			fmt.Printf("Warning: plugin %s has invalid NewPlugin signature\n", entry.Name())
			continue
		}

		loaded = append(loaded, newPlugin())
	}

	return loaded, nil
}
