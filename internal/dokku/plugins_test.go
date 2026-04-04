package dokku

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetServicePluginList(t *testing.T) {
	t.Run("Returns complete service plugin list", func(t *testing.T) {
		plugins := GetServicePluginList()

		// Check that we have expected number of plugins
		assert.Greater(t, len(plugins), 10, "Should have more than 10 service plugins")

		// Check for specific plugins
		expectedPlugins := []string{
			"postgres",
			"mariadb",
			"mongo",
			"redis",
			"rabbitmq",
			"memcached",
			"elasticsearch",
		}

		for _, expected := range expectedPlugins {
			assert.Contains(t, plugins, expected, "Should contain %s", expected)
		}
	})

	t.Run("Returns consistent results", func(t *testing.T) {
		list1 := GetServicePluginList()
		list2 := GetServicePluginList()

		assert.Equal(t, list1, list2, "Should return same list on multiple calls")
	})
}

func TestIsPluginInstalledWithRunner(t *testing.T) {
	tests := []struct {
		name           string
		pluginName     string
		output         string
		commandError   error
		expectedResult bool
		expectedError  bool
	}{
		{
			name:       "plugin is installed",
			pluginName: "postgres",
			output: `postgres                     1.23.1 enabled    dokku postgres service plugin
mysql                        1.20.0 enabled    dokku mysql service plugin`,
			commandError:   nil,
			expectedResult: true,
			expectedError:  false,
		},
		{
			name:           "plugin not installed",
			pluginName:     "redis",
			output:         "postgres                     1.23.1 enabled    dokku postgres service plugin",
			commandError:   nil,
			expectedResult: false,
			expectedError:  false,
		},
		{
			name:           "command error",
			pluginName:     "postgres",
			output:         "",
			commandError:   errors.New("command failed"),
			expectedResult: false,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRunner := &mockCommandRunner{
				runCommandFunc: func(command string, args ...string) (string, error) {
					if command == "dokku" && len(args) > 0 && args[0] == "plugin:list" {
						if tt.commandError != nil {
							return "", tt.commandError
						}
						return tt.output, nil
					}
					return "", nil
				},
			}

			installed, err := IsPluginInstalledWithRunner(mockRunner, tt.pluginName)

			if tt.expectedError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectedError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if installed != tt.expectedResult {
				t.Errorf("Expected installed=%v, got %v", tt.expectedResult, installed)
			}
		})
	}
}

func TestGetPlugins(t *testing.T) {
	mockOutput := `postgres                     1.23.1 enabled    dokku postgres service plugin
mysql                        1.20.0 enabled    dokku mysql service plugin
redis                        1.18.0 disabled   dokku redis service plugin`

	mockRunner := &mockCommandRunner{
		runCommandFunc: func(command string, args ...string) (string, error) {
			if command == "dokku" && len(args) > 0 && args[0] == "plugin:list" {
				return mockOutput, nil
			}
			if command == "dokku" && len(args) > 0 && args[0] == "--version" {
				return "dokku version 0.34.4", nil
			}
			return "", nil
		},
	}

	plugins, err := GetPlugins(mockRunner)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(plugins) != 3 {
		t.Errorf("Expected 3 plugins, got %d", len(plugins))
	}

	// Check first plugin
	if plugins[0].Name != "postgres" {
		t.Errorf("Expected first plugin name 'postgres', got '%s'", plugins[0].Name)
	}
	if plugins[0].Version != "1.23.1" {
		t.Errorf("Expected first plugin version '1.23.1', got '%s'", plugins[0].Version)
	}

	// Check second plugin
	if plugins[1].Name != "mysql" {
		t.Errorf("Expected second plugin name 'mysql', got '%s'", plugins[1].Name)
	}
	if plugins[1].Version != "1.20.0" {
		t.Errorf("Expected second plugin version '1.20.0', got '%s'", plugins[1].Version)
	}
}

func TestGetPlugins_Error(t *testing.T) {
	mockRunner := &mockCommandRunner{
		runCommandFunc: func(command string, args ...string) (string, error) {
			return "", errors.New("command failed")
		},
	}

	_, err := GetPlugins(mockRunner)
	if err == nil {
		t.Error("Expected error when command fails")
	}
}

func TestGetAvailablePlugins(t *testing.T) {
	tests := []struct {
		name              string
		installedOutput   string
		commandError      error
		expectedAvailable int
		expectedError     bool
	}{
		{
			name: "some plugins installed",
			installedOutput: `postgres                     1.23.1 enabled    dokku postgres service plugin
letsencrypt                  0.19.0 enabled    dokku letsencrypt plugin`,
			commandError:      nil,
			expectedAvailable: 17, // Total 19 - 2 installed
			expectedError:     false,
		},
		{
			name:              "no plugins installed",
			installedOutput:   "",
			commandError:      nil,
			expectedAvailable: 19, // All plugins available
			expectedError:     false,
		},
		{
			name:              "command error",
			installedOutput:   "",
			commandError:      errors.New("command failed"),
			expectedAvailable: 0,
			expectedError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRunner := &mockCommandRunner{
				runCommandFunc: func(command string, args ...string) (string, error) {
					if tt.commandError != nil {
						return "", tt.commandError
					}
					if command == "dokku" && len(args) > 0 && args[0] == "plugin:list" {
						return tt.installedOutput, nil
					}
					return "", nil
				},
			}

			plugins, err := GetAvailablePlugins(mockRunner)

			if tt.expectedError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(plugins) != tt.expectedAvailable {
				t.Errorf("Expected %d available plugins, got %d", tt.expectedAvailable, len(plugins))
			}
		})
	}
}

func TestGetInstalledPluginsMap(t *testing.T) {
	tests := []struct {
		name          string
		output        string
		commandError  error
		expectedMap   map[string]bool
		expectedError bool
	}{
		{
			name: "multiple plugins",
			output: `postgres                     1.23.1 enabled    dokku postgres service plugin
mysql                        1.20.0 enabled    dokku mysql service plugin
redis                        1.18.0 disabled   dokku redis service plugin`,
			expectedMap: map[string]bool{
				"postgres": true,
				"mysql":    true,
				"redis":    true,
			},
			expectedError: false,
		},
		{
			name:          "no plugins",
			output:        "",
			expectedMap:   map[string]bool{},
			expectedError: false,
		},
		{
			name:          "command error",
			output:        "",
			commandError:  errors.New("command failed"),
			expectedMap:   nil,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRunner := &mockCommandRunner{
				runCommandFunc: func(command string, args ...string) (string, error) {
					if tt.commandError != nil {
						return "", tt.commandError
					}
					if command == "dokku" && len(args) > 0 && args[0] == "plugin:list" {
						return tt.output, nil
					}
					return "", nil
				},
			}

			pluginMap, err := GetInstalledPluginsMap(mockRunner)

			if tt.expectedError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(pluginMap) != len(tt.expectedMap) {
				t.Errorf("Expected %d plugins in map, got %d", len(tt.expectedMap), len(pluginMap))
			}

			for key, expectedVal := range tt.expectedMap {
				if val, exists := pluginMap[key]; !exists || val != expectedVal {
					t.Errorf("Expected plugin[%s]=%v, got %v (exists: %v)", key, expectedVal, val, exists)
				}
			}
		})
	}
}

func TestGetDatabasePluginList(t *testing.T) {
	t.Run("Returns database plugin list", func(t *testing.T) {
		plugins := GetDatabasePluginList()

		// Check that we have expected databases
		expectedDatabases := []string{
			"postgres",
			"mariadb",
			"mongo",
			"redis",
			"couchdb",
			"rethinkdb",
		}

		assert.Equal(t, len(expectedDatabases), len(plugins), "Should have exactly 6 database plugins")

		for _, expected := range expectedDatabases {
			assert.Contains(t, plugins, expected, "Should contain %s", expected)
		}
	})

	t.Run("Database list is subset of service list", func(t *testing.T) {
		servicePlugins := GetServicePluginList()
		databasePlugins := GetDatabasePluginList()

		// Every database plugin should be in the service list
		for _, dbPlugin := range databasePlugins {
			assert.Contains(t, servicePlugins, dbPlugin, "Database plugin %s should be in service list", dbPlugin)
		}
	})
}

// Mock CommandRunner for testing
type mockCommandRunner struct {
	runCommandFunc func(command string, args ...string) (string, error)
}

func (m *mockCommandRunner) RunCommand(command string, args ...string) (string, error) {
	if m.runCommandFunc != nil {
		return m.runCommandFunc(command, args...)
	}
	return "", nil
}

func (m *mockCommandRunner) StartPTY(command string, args ...string) (*os.File, error) {
	// Not needed for these tests
	return nil, nil
}

func TestGetAvailableServicePluginList(t *testing.T) {
	t.Run("Returns installed service plugins", func(t *testing.T) {
		mockRunner := &mockCommandRunner{
			runCommandFunc: func(command string, args ...string) (string, error) {
				if command == "dokku" && len(args) > 0 && args[0] == "plugin:list" {
					return `postgres            1.29.3 enabled    dokku postgres service plugin
redis               1.28.2 enabled    dokku redis service plugin
mariadb             1.27.1 enabled    dokku mariadb service plugin`, nil
				}
				return "", nil
			},
		}

		plugins, err := GetAvailableServicePluginList(mockRunner)
		assert.NoError(t, err)
		assert.Contains(t, plugins, "postgres")
		assert.Contains(t, plugins, "redis")
		assert.Contains(t, plugins, "mariadb")
		assert.NotContains(t, plugins, "mongo")
	})

	t.Run("Returns empty list when no plugins installed", func(t *testing.T) {
		mockRunner := &mockCommandRunner{
			runCommandFunc: func(command string, args ...string) (string, error) {
				if command == "dokku" && len(args) > 0 && args[0] == "plugin:list" {
					return "", nil
				}
				return "", nil
			},
		}

		plugins, err := GetAvailableServicePluginList(mockRunner)
		assert.NoError(t, err)
		assert.Empty(t, plugins)
	})

	t.Run("Returns error when command fails", func(t *testing.T) {
		mockRunner := &mockCommandRunner{
			runCommandFunc: func(command string, args ...string) (string, error) {
				return "", errors.New("command failed")
			},
		}

		plugins, err := GetAvailableServicePluginList(mockRunner)
		assert.Error(t, err)
		assert.Nil(t, plugins)
	})
}

func TestGetAvailableDatabasePluginList(t *testing.T) {
	t.Run("Returns only database plugins that are installed", func(t *testing.T) {
		mockRunner := &mockCommandRunner{
			runCommandFunc: func(command string, args ...string) (string, error) {
				if command == "dokku" && len(args) > 0 && args[0] == "plugin:list" {
					return `postgres            1.29.3 enabled    dokku postgres service plugin
redis               1.28.2 enabled    dokku redis service plugin
elasticsearch       1.26.1 enabled    dokku elasticsearch service plugin`, nil
				}
				return "", nil
			},
		}

		plugins, err := GetAvailableDatabasePluginList(mockRunner)
		assert.NoError(t, err)

		// postgres and redis are databases and installed
		assert.Contains(t, plugins, "postgres")
		assert.Contains(t, plugins, "redis")

		// elasticsearch is not a database
		assert.NotContains(t, plugins, "elasticsearch")
	})

	t.Run("Returns empty list when no database plugins installed", func(t *testing.T) {
		mockRunner := &mockCommandRunner{
			runCommandFunc: func(command string, args ...string) (string, error) {
				if command == "dokku" && len(args) > 0 && args[0] == "plugin:list" {
					return `rabbitmq            1.26.1 enabled    dokku rabbitmq service plugin
memcached           1.25.0 enabled    dokku memcached service plugin`, nil
				}
				return "", nil
			},
		}

		plugins, err := GetAvailableDatabasePluginList(mockRunner)
		assert.NoError(t, err)
		assert.Empty(t, plugins, "Should be empty as no database plugins are installed")
	})

	t.Run("Returns error when command fails", func(t *testing.T) {
		mockRunner := &mockCommandRunner{
			runCommandFunc: func(command string, args ...string) (string, error) {
				return "", errors.New("command failed")
			},
		}

		plugins, err := GetAvailableDatabasePluginList(mockRunner)
		assert.Error(t, err)
		assert.Nil(t, plugins)
	})
}

func BenchmarkGetServicePluginList(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GetServicePluginList()
	}
}

func BenchmarkGetDatabasePluginList(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GetDatabasePluginList()
	}
}
