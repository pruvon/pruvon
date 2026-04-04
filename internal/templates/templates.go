package templates

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"github.com/pruvon/pruvon/internal/config"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//go:embed html/*.html html/partials/*.html html/partials/app_detail/*.html html/settings/*.html
var templatesFS embed.FS

var components []string
var templateCache = make(map[string]*template.Template)

func useLocalTemplates() bool {
	_, err := os.Stat("internal/templates/html")
	return err == nil
}

// Initialize initializes the templates
func Initialize() error {
	// Get a list of all root partials
	rootPartials, err := templatesFS.ReadDir("html/partials")
	if err == nil {
		for _, entry := range rootPartials {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".html") {
				components = append(components, "html/partials/"+entry.Name())
			}
		}
	}

	// Get a list of all app_detail partials
	appDetailPartials, err := templatesFS.ReadDir("html/partials/app_detail")
	if err == nil {
		for _, entry := range appDetailPartials {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".html") {
				components = append(components, "html/partials/app_detail/"+entry.Name())
			}
		}
	}

	// Look for local templates directory
	localTemplatesDir := "internal/templates/html"
	if _, err := os.Stat(localTemplatesDir); err == nil {
		// If local templates directory exists, use it for development
		if err := filepath.Walk(localTemplatesDir, func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Skip directories and non-HTML files
			if info.IsDir() || !strings.HasSuffix(info.Name(), ".html") {
				return nil
			}

			// Remove the "templates/" prefix from the path to match the embed paths
			relativePath := strings.TrimPrefix(path, "internal/templates/")

			// Check if this is a component file
			if strings.HasPrefix(relativePath, "html/components/") {
				// Replace the component in the components list if it exists
				for i, component := range components {
					if strings.HasSuffix(component, info.Name()) {
						components[i] = relativePath
						break
					}
				}
				// If it's a new component, add it
				newComponent := true
				for _, component := range components {
					if strings.HasSuffix(component, info.Name()) {
						newComponent = false
						break
					}
				}
				if newComponent {
					components = append(components, relativePath)
				}
			}

			return nil
		}); err != nil {
			return err
		}
	}

	return nil
}

// GetTemplate returns a template with the specified name
func GetTemplate(name string) (*template.Template, error) {
	cacheEnabled := !useLocalTemplates()

	// Check cache first
	if cacheEnabled {
		if tmpl, ok := templateCache[name]; ok {
			return tmpl, nil
		}
	}

	// Get the template content
	templateContent, err := getTemplateContent("html/" + name)
	if err != nil {
		return nil, err
	}

	// Get the base template content
	baseContent, err := getTemplateContent("html/base.html")
	if err != nil {
		return nil, err
	}

	// Create a new template with the base content
	tmpl, err := template.New("base.html").Funcs(getFuncMap()).Parse(string(baseContent))
	if err != nil {
		return nil, err
	}

	// Parse the component templates
	for _, component := range components {
		componentContent, err := getTemplateContent(component)
		if err != nil {
			continue
		}
		// Use the base filename as the template name
		templateName := filepath.Base(component)
		if _, err := tmpl.New(templateName).Parse(string(componentContent)); err != nil {
			continue
		}
	}

	// Parse the main template
	if _, err := tmpl.New(name).Parse(string(templateContent)); err != nil {
		return nil, err
	}

	// Cache embedded templates, but always re-read local disk templates in development.
	if cacheEnabled {
		templateCache[name] = tmpl
	}

	return tmpl, nil
}

// getTemplateContent retrieves the content of a template file
func getTemplateContent(path string) ([]byte, error) {
	// First check if the file exists on disk (for development)
	if _, err := os.Stat("internal/templates/" + path); err == nil {
		content, err := os.ReadFile("internal/templates/" + path)
		if err != nil {
			return nil, err
		}
		return content, nil
	}

	// If not on disk, retrieve from the embedded filesystem
	content, err := templatesFS.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return content, nil
}

// getFuncMap returns the function map for templates
func getFuncMap() template.FuncMap {
	// Create function map
	return template.FuncMap{
		"json":                   jsonFunc,
		"hasRouteAccess":         hasRouteAccess,
		"hasAppAccess":           hasAppAccess,
		"hasServiceAccess":       hasServiceAccess,
		"getUserAllowedApps":     getUserAllowedApps,
		"getUserAllowedServices": getUserAllowedServices,
		"formatDate":             formatDate,
	}
}

// formatDate formats a time.Time to DD.MM.YYYY HH:MM format
func formatDate(t time.Time) string {
	return t.Format("02.01.2006 15:04")
}

// RenderToString renders a template with data to a string
func RenderToString(templateName string, data interface{}) (string, error) {
	tmpl, err := GetTemplate(templateName)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "base.html", data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// hasRouteAccess checks if a user has access to a given route
func hasRouteAccess(username interface{}, route string, authType interface{}) bool {
	// Admin has access to everything
	if authType == "admin" {
		return true
	}

	// For GitHub users, check their routes
	if authType == "github" {
		// Handle nil username
		if username == nil {
			return false
		}

		// Convert username to string
		usernameStr := ""
		switch v := username.(type) {
		case string:
			usernameStr = v
		default:
			return false
		}

		// Get config
		cfg := config.GetConfig()
		if cfg == nil {
			return false
		}

		// Check user permissions
		for _, user := range cfg.GitHub.Users {
			if user.Username == usernameStr {
				// Check if the user has access to the route
				for _, r := range user.Routes {
					if r == "*" || r == "/*" {
						return true
					}

					// Exact match
					if r == route {
						return true
					}

					// Wildcard match
					// e.g. /apps/* matches /apps/myapp
					if strings.HasSuffix(r, "/*") && strings.HasPrefix(route, r[:len(r)-1]) {
						return true
					}
				}

				// Check if the route is for an app the user has access to
				if strings.HasPrefix(route, "/apps/") {
					appName := strings.TrimPrefix(route, "/apps/")
					if strings.Contains(appName, "/") {
						appName = strings.Split(appName, "/")[0]
					}

					for _, app := range user.Apps {
						if app == "*" || app == appName {
							return true
						}
					}
				}
			}
		}
	}

	return false
}

func jsonFunc(value interface{}) template.JS {
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return ""
	}
	return template.JS(jsonBytes)
}

// hasAppAccess checks if a user has access to any app or a specific app
func hasAppAccess(username interface{}, specificApp string, authType interface{}) bool {
	// Admin has access to everything
	if authType == "admin" {
		return true
	}

	// For GitHub users, check their app permissions
	if authType == "github" {
		// Handle nil username
		if username == nil {
			return false
		}

		// Convert username to string
		usernameStr := ""
		switch v := username.(type) {
		case string:
			usernameStr = v
		default:
			return false
		}

		// Get config
		cfg := config.GetConfig()
		if cfg == nil {
			return false
		}

		// Check user permissions
		for _, user := range cfg.GitHub.Users {
			if user.Username == usernameStr {
				// Check for wildcard access in routes
				for _, r := range user.Routes {
					if r == "*" || r == "/*" || r == "/apps/*" {
						return true
					}
				}

				// Check if user has any app access
				if len(user.Apps) > 0 {
					// If no specific app is requested, any app permission is enough
					if specificApp == "" {
						return true
					}

					// If specific app requested, check if it's in the allowed list
					for _, app := range user.Apps {
						if app == "*" || app == specificApp {
							return true
						}
					}
				}

				return false
			}
		}
	}

	return false
}

// hasServiceAccess checks if a user has access to any service or a specific service
func hasServiceAccess(username interface{}, specificType string, specificName string, authType interface{}) bool {
	// Admin has access to everything
	if authType == "admin" {
		return true
	}

	// For GitHub users, check their service permissions
	if authType == "github" {
		// Handle nil username
		if username == nil {
			return false
		}

		// Convert username to string
		usernameStr := ""
		switch v := username.(type) {
		case string:
			usernameStr = v
		default:
			return false
		}

		// Get config
		cfg := config.GetConfig()
		if cfg == nil {
			return false
		}

		// Check user permissions
		for _, user := range cfg.GitHub.Users {
			if user.Username == usernameStr {
				// Check for wildcard access in routes
				for _, r := range user.Routes {
					if r == "*" || r == "/*" || r == "/services/*" {
						return true
					}
				}

				// If no services defined, user has no access
				if len(user.Services) == 0 {
					return false
				}

				// If no specific service is requested, any service permission is enough
				if specificType == "" && specificName == "" {
					for _, svcList := range user.Services {
						if len(svcList) > 0 {
							return true
						}
					}
					return false
				}

				// If specific type requested but no name, check if user has any service of that type
				if specificType != "" && specificName == "" {
					if svcList, ok := user.Services[specificType]; ok && len(svcList) > 0 {
						return true
					}
					return false
				}

				// If specific service requested, check if it's in the allowed list
				if specificType != "" && specificName != "" {
					if svcList, ok := user.Services[specificType]; ok {
						for _, svc := range svcList {
							if svc == "*" || svc == specificName {
								return true
							}
						}
					}
				}

				return false
			}
		}
	}

	return false
}

// getUserAllowedApps returns a list of app names that the user has access to
func getUserAllowedApps(username interface{}, authType interface{}, allApps []string) []string {
	// Admin has access to everything
	if authType == "admin" {
		return allApps
	}

	// For GitHub users, filter based on permissions
	if authType == "github" {
		// Handle nil username
		if username == nil {
			return []string{}
		}

		// Convert username to string
		usernameStr, ok := username.(string)
		if !ok {
			return []string{}
		}

		// Get config
		cfg := config.GetConfig()
		if cfg == nil {
			return []string{}
		}

		// Create a map for faster lookup of existing apps
		existingAppsMap := make(map[string]bool)
		for _, app := range allApps {
			existingAppsMap[app] = true
		}

		// Find the user
		for _, user := range cfg.GitHub.Users {
			if user.Username == usernameStr {
				// Check for wildcard access in routes or apps
				for _, r := range user.Routes {
					if r == "*" || r == "/*" || r == "/apps/*" {
						return allApps
					}
				}

				// Check if user has wildcard app access
				for _, app := range user.Apps {
					if app == "*" {
						return allApps
					}
				}

				// Initialize the allowed apps list
				allowedApps := []string{}

				// Add apps from the Apps property
				if len(user.Apps) > 0 {
					for _, app := range user.Apps {
						if app != "*" {
							// Add the app even if it doesn't exist yet - this handles the case
							// where the app may be created later but permissions already exist
							allowedApps = append(allowedApps, app)
						}
					}
				}

				// Also check routes for app-specific permissions
				if len(user.Routes) > 0 {
					for _, route := range user.Routes {
						if strings.HasPrefix(route, "/apps/") && route != "/apps" && route != "/apps/*" {
							// Extract app name from route
							appName := strings.TrimPrefix(route, "/apps/")
							// Remove trailing "/*" if present
							appName = strings.TrimSuffix(appName, "/*")

							// Check if this app isn't already in our list
							alreadyExists := false
							for _, allowed := range allowedApps {
								if allowed == appName {
									alreadyExists = true
									break
								}
							}
							if !alreadyExists && appName != "" {
								allowedApps = append(allowedApps, appName)
							}
						}
					}
				}

				return allowedApps
			}
		}

		// User not found
		return []string{}
	}

	// Unknown auth type
	return []string{}
}

// GetUserAllowedApps is an exported wrapper for getUserAllowedApps
func GetUserAllowedApps(username interface{}, authType interface{}, allApps []string) []string {
	return getUserAllowedApps(username, authType, allApps)
}

// getUserAllowedServices returns a list of service names that the user has access to for a given type
func getUserAllowedServices(username interface{}, authType interface{}, svcType string, allServices []string) []string {
	// Admin has access to everything
	if authType == "admin" {
		return allServices
	}

	// For GitHub users, filter based on permissions
	if authType == "github" {
		// Handle nil username
		if username == nil {
			return []string{}
		}

		// Convert username to string
		usernameStr, ok := username.(string)
		if !ok {
			return []string{}
		}

		// Get config
		cfg := config.GetConfig()
		if cfg == nil {
			return []string{}
		}

		// Find the user
		for _, user := range cfg.GitHub.Users {
			if user.Username == usernameStr {
				// Check for wildcard access in routes
				for _, r := range user.Routes {
					if r == "*" || r == "/*" || r == "/services/*" {
						return allServices
					}
				}

				// Check service permissions
				if user.Services != nil { // Services use the same Services field for permissions
					// Check if user has wildcard access to this service type
					if svcList, ok := user.Services[svcType]; ok {
						for _, svc := range svcList {
							if svc == "*" {
								return allServices
							}
						}

						// User has specific service permissions, return those services
						allowedSvcs := []string{}

						// Include all services from user permissions
						for _, svc := range svcList {
							if svc != "*" {
								// Add the service even if it doesn't exist yet in the system
								allowedSvcs = append(allowedSvcs, svc)
							}
						}

						return allowedSvcs
					}
				}

				// If we reach here, the user has no service permissions for this type
				return []string{}
			}
		}

		// User not found
		return []string{}
	}

	// Unknown auth type
	return []string{}
}

// GetUserAllowedServices is an exported wrapper for getUserAllowedServices
func GetUserAllowedServices(username interface{}, authType interface{}, svcType string, allServices []string) []string {
	return getUserAllowedServices(username, authType, svcType, allServices)
}

// TemplateExists checks if a template exists
func TemplateExists(name string) bool {
	// First try to get the template from embedded templates
	embeddedPath := filepath.Join("html", name)
	_, err := templatesFS.Open(embeddedPath)
	if err == nil {
		return true
	}

	// Then try to get the template from local file system
	localPath := filepath.Join("internal", "templates", "html", name)
	_, err = os.Stat(localPath)
	return err == nil
}

// ClearCache clears the template cache to force reloading of templates
func ClearCache() {
	templateCache = make(map[string]*template.Template)
}
