package dokku

import (
	"strings"

	"github.com/pruvon/pruvon/internal/models"
)

// IsPluginInstalled checks if a dokku plugin is installed by executing the command directly
func IsPluginInstalled(pluginName string) (bool, error) {
	return IsPluginInstalledWithRunner(DefaultCommandRunner, pluginName)
}

// IsPluginInstalledWithRunner checks if a dokku plugin is installed using the provided command runner
func IsPluginInstalledWithRunner(runner CommandRunner, pluginName string) (bool, error) {
	output, err := runner.RunCommand("dokku", "plugin:list")
	if err != nil {
		return false, err
	}

	return strings.Contains(output, pluginName), nil
}

// GetPlugins returns a list of installed Dokku plugins
func GetPlugins(runner CommandRunner) ([]models.Plugin, error) {
	dokkuVersion, err := GetDokkuVersion(runner)
	if err != nil {
		return nil, err
	}

	output, err := runner.RunCommand("dokku", "plugin:list")
	if err != nil {
		return nil, err
	}

	lines := strings.Split(output, "\n")
	var plugins []models.Plugin
	for _, line := range lines {
		if line == "" || strings.Contains(line, "=====") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			// Eğer plugin sürümü dokku sürümünden farklıysa listeye ekle
			if parts[1] != dokkuVersion {
				plugins = append(plugins, models.Plugin{
					Name:    parts[0],
					Version: parts[1],
				})
			}
		}
	}
	return plugins, nil
}

// GetAvailablePlugins returns a list of available Dokku plugins that are not installed
func GetAvailablePlugins(runner CommandRunner) ([]models.AvailablePlugin, error) {
	// Get installed plugins
	installedPlugins := make(map[string]bool)
	output, err := runner.RunCommand("dokku", "plugin:list")
	if err != nil {
		return nil, err
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if line == "" || strings.Contains(line, "=====") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 1 {
			installedPlugins[parts[0]] = true
		}
	}

	// Define all possible plugins
	allPlugins := []models.AvailablePlugin{
		{Name: "letsencrypt", URL: "https://github.com/dokku/dokku-letsencrypt.git"},
		{Name: "mariadb", URL: "https://github.com/dokku/dokku-mariadb.git"},
		{Name: "postgres", URL: "https://github.com/dokku/dokku-postgres.git"},
		{Name: "redis", URL: "https://github.com/dokku/dokku-redis.git"},
		{Name: "rabbitmq", URL: "https://github.com/dokku/dokku-rabbitmq.git"},
		{Name: "memcached", URL: "https://github.com/dokku/dokku-memcached.git"},
		{Name: "mongo", URL: "https://github.com/dokku/dokku-mongo.git"},
		{Name: "maintenance", URL: "https://github.com/dokku/dokku-maintenance.git"},
		{Name: "http-auth", URL: "https://github.com/dokku/dokku-http-auth.git"},
		{Name: "redirect", URL: "https://github.com/dokku/dokku-redirect.git"},
		{Name: "clickhouse", URL: "https://github.com/dokku/dokku-clickhouse.git"},
		{Name: "elasticsearch", URL: "https://github.com/dokku/dokku-elasticsearch.git"},
		{Name: "meilisearch", URL: "https://github.com/dokku/dokku-meilisearch.git"},
		{Name: "nats", URL: "https://github.com/dokku/dokku-nats.git"},
		{Name: "pushpin", URL: "https://github.com/dokku/dokku-pushpin.git"},
		{Name: "omnisci", URL: "https://github.com/dokku/dokku-omnisci.git"},
		{Name: "rethinkdb", URL: "https://github.com/dokku/dokku-rethinkdb.git"},
		{Name: "solr", URL: "https://github.com/dokku/dokku-solr.git"},
		{Name: "typesense", URL: "https://github.com/dokku/dokku-typesense.git"},
	}

	// Filter out installed plugins
	var availablePlugins []models.AvailablePlugin
	for _, plugin := range allPlugins {
		// Extract name from URL for comparison
		urlParts := strings.Split(plugin.URL, "/")
		repoName := urlParts[len(urlParts)-1]
		name := strings.TrimPrefix(strings.TrimSuffix(repoName, ".git"), "dokku-")
		if idx := strings.Index(name, "-"); idx != -1 {
			name = name[:idx]
		}

		if !installedPlugins[name] {
			availablePlugins = append(availablePlugins, plugin)
		}
	}

	return availablePlugins, nil
}

// GetInstalledPluginsMap returns a map of installed plugins for efficient lookups.
func GetInstalledPluginsMap(runner CommandRunner) (map[string]bool, error) {
	output, err := runner.RunCommand("dokku", "plugin:list")
	if err != nil {
		return nil, err
	}

	installedPlugins := make(map[string]bool)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if line == "" || strings.Contains(line, "=====") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 1 {
			installedPlugins[parts[0]] = true
		}
	}
	return installedPlugins, nil
}

// GetServicePluginList returns the complete list of all service plugins supported by Dokku
func GetServicePluginList() []string {
	return []string{
		"postgres",
		"mariadb",
		"mongo",
		"redis",
		"rabbitmq",
		"memcached",
		"clickhouse",
		"elasticsearch",
		"nats",
		"solr",
		"rethinkdb",
		"couchdb",
		"meilisearch",
		"pushpin",
		"omnisci",
	}
}

// GetDatabasePluginList returns the complete list of database plugins supported by Dokku
func GetDatabasePluginList() []string {
	return []string{
		"postgres",
		"mariadb",
		"mongo",
		"redis",
		"couchdb",
		"rethinkdb",
	}
}

// GetAvailableServicePluginList returns the list of installed service plugins
func GetAvailableServicePluginList(runner CommandRunner) ([]string, error) {
	allPlugins := GetServicePluginList()
	installedPlugins, err := GetInstalledPluginsMap(runner)
	if err != nil {
		return nil, err
	}

	var availablePlugins []string
	for _, plugin := range allPlugins {
		if installedPlugins[plugin] {
			availablePlugins = append(availablePlugins, plugin)
		}
	}

	return availablePlugins, nil
}

// GetAvailableDatabasePluginList returns the list of installed database plugins
func GetAvailableDatabasePluginList(runner CommandRunner) ([]string, error) {
	allPlugins := GetDatabasePluginList()
	installedPlugins, err := GetInstalledPluginsMap(runner)
	if err != nil {
		return nil, err
	}

	var availablePlugins []string
	for _, plugin := range allPlugins {
		if installedPlugins[plugin] {
			availablePlugins = append(availablePlugins, plugin)
		}
	}

	return availablePlugins, nil
}
