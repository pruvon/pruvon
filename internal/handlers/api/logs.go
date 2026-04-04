package api

import (
	"github.com/pruvon/pruvon/internal/handlers/common"
	"github.com/pruvon/pruvon/internal/models"
	servicelogs "github.com/pruvon/pruvon/internal/services/logs"

	"github.com/gofiber/fiber/v2"
)

func SetupLogsRoutes(app *fiber.App) {
	app.Get("/api/logs", handleGetLogs)
}

func handleGetLogs(c *fiber.Ctx) error {
	params := models.LogSearchParams{
		Username: c.Query("username"),
		Query:    c.Query("q"),
		Page:     1,
		PerPage:  20,
	}

	if page := c.QueryInt("page"); page > 0 {
		params.Page = page
	}

	result, err := servicelogs.SearchLogs(params)
	if err != nil {
		return common.ErrorResponse(c, fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(result)
}
