package templates

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"github.com/pruvon/pruvon/internal/config"
	"github.com/pruvon/pruvon/internal/middleware/authz"
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

	user := lookupScopedUser(username)
	if user != nil {
		if route == "/apps" && userHasAnyAppAccess(user) {
			return true
		}

		for _, r := range user.Routes {
			if r == "*" || r == "/*" {
				return true
			}

			if r == route {
				return true
			}

			if strings.HasSuffix(r, "/*") && strings.HasPrefix(route, r[:len(r)-1]) {
				return true
			}
		}

		if strings.HasPrefix(route, "/apps/") {
			appName := strings.TrimPrefix(route, "/apps/")
			if strings.Contains(appName, "/") {
				appName = strings.Split(appName, "/")[0]
			}

			if userHasExplicitAppAccess(user, appName) || userHasRouteDerivedAppAccess(user, appName) {
				return true
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

	user := lookupScopedUser(username)
	if user != nil {
		for _, r := range user.Routes {
			if r == "*" || r == "/*" || r == "/apps/*" {
				return true
			}
		}

		if specificApp == "" {
			return userHasAnyAppAccess(user)
		}

		if userHasExplicitAppAccess(user, specificApp) || userHasRouteDerivedAppAccess(user, specificApp) {
			return true
		}

		return false
	}

	return false
}

// hasServiceAccess checks if a user has access to any service or a specific service
func hasServiceAccess(username interface{}, specificType string, specificName string, authType interface{}) bool {
	// Admin has access to everything
	if authType == "admin" {
		return true
	}

	user := lookupScopedUser(username)
	if user != nil {
		for _, r := range user.Routes {
			if r == "*" || r == "/*" || r == "/services/*" {
				return true
			}
		}

		if len(user.Services) == 0 {
			return false
		}

		if specificType == "" && specificName == "" {
			for _, svcList := range user.Services {
				if len(svcList) > 0 {
					return true
				}
			}
			return false
		}

		if specificType != "" && specificName == "" {
			if svcList, ok := user.Services[specificType]; ok && len(svcList) > 0 {
				return true
			}
			return false
		}

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

	return false
}

// getUserAllowedApps returns a list of app names that the user has access to
func getUserAllowedApps(username interface{}, authType interface{}, allApps []string) []string {
	// Admin has access to everything
	if authType == "admin" {
		return allApps
	}

	user := lookupScopedUser(username)
	if user != nil {
		for _, r := range user.Routes {
			if r == "*" || r == "/*" || r == "/apps/*" {
				return allApps
			}
		}

		for _, app := range user.Apps {
			if app == "*" {
				return allApps
			}
		}

		allowedApps := make([]string, 0)
		seenApps := make(map[string]bool)

		for _, app := range user.Apps {
			if app == "*" || app == "" || seenApps[app] {
				continue
			}
			seenApps[app] = true
			allowedApps = append(allowedApps, app)
		}

		for _, route := range user.Routes {
			if !strings.HasPrefix(route, "/apps/") || route == "/apps" || route == "/apps/*" {
				continue
			}

			appPattern := strings.TrimPrefix(route, "/apps/")
			if appPattern == "" {
				continue
			}

			if strings.HasSuffix(appPattern, "/*") {
				exactApp := strings.TrimSuffix(appPattern, "/*")
				if authz.RouteGrantsApp(route, exactApp) && !seenApps[exactApp] {
					seenApps[exactApp] = true
					allowedApps = append(allowedApps, exactApp)
				}
				continue
			}

			if strings.HasSuffix(appPattern, "*") {
				for _, app := range allApps {
					if app == "" || seenApps[app] || !authz.RouteGrantsApp(route, app) {
						continue
					}
					seenApps[app] = true
					allowedApps = append(allowedApps, app)
				}
				continue
			}

			if authz.RouteGrantsApp(route, appPattern) && !seenApps[appPattern] {
				seenApps[appPattern] = true
				allowedApps = append(allowedApps, appPattern)
			}
		}

		return allowedApps
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

	user := lookupScopedUser(username)
	if user != nil {
		for _, r := range user.Routes {
			if r == "*" || r == "/*" || r == "/services/*" {
				return allServices
			}
		}

		if user.Services != nil {
			if svcList, ok := user.Services[svcType]; ok {
				for _, svc := range svcList {
					if svc == "*" {
						return allServices
					}
				}

				allowedSvcs := []string{}
				for _, svc := range svcList {
					if svc != "*" {
						allowedSvcs = append(allowedSvcs, svc)
					}
				}

				return allowedSvcs
			}
		}

		return []string{}
	}

	// Unknown auth type
	return []string{}
}

func lookupScopedUser(username interface{}) *config.User {
	if username == nil {
		return nil
	}
	usernameStr, ok := username.(string)
	if !ok || usernameStr == "" {
		return nil
	}
	user := config.FindUserByUsername(usernameStr)
	if user == nil || user.Disabled {
		return nil
	}
	return user
}

func userHasAnyAppAccess(user *config.User) bool {
	if len(user.Apps) > 0 {
		return true
	}

	for _, route := range user.Routes {
		if authz.RouteGrantsAnyApp(route) {
			return true
		}
	}

	return false
}

func userHasExplicitAppAccess(user *config.User, appName string) bool {
	for _, app := range user.Apps {
		if app == "*" || app == appName {
			return true
		}
	}

	return false
}

func userHasRouteDerivedAppAccess(user *config.User, appName string) bool {
	for _, route := range user.Routes {
		if authz.RouteGrantsApp(route, appName) {
			return true
		}
	}

	return false
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
