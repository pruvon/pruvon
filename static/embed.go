package static

import "embed"

//go:embed css js images images/logo/*.svg
var StaticFiles embed.FS

// Create a separate variable for embedded templates
// Templates will need to be copied to the static directory
// in the build process or deployment pipeline
//
//go:embed app_templates/*.json
var EmbeddedTemplates embed.FS
