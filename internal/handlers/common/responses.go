package common

import "github.com/gofiber/fiber/v2"

// ErrorResponse sends a standardized JSON error payload with optional details.
func ErrorResponse(c *fiber.Ctx, statusCode int, message string, details ...interface{}) error {
	payload := fiber.Map{
		"error": message,
	}

	switch len(details) {
	case 1:
		payload["details"] = details[0]
	case 0:
		// no-op
	default:
		payload["details"] = details
	}

	return c.Status(statusCode).JSON(payload)
}

// SuccessResponse sends a standardized JSON success payload.
func SuccessResponse(c *fiber.Ctx, data interface{}) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": data,
	})
}

// ValidationErrorResponse sends a standardized validation error payload.
func ValidationErrorResponse(c *fiber.Ctx, errors map[string]string) error {
	return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
		"error":   "validation_error",
		"details": errors,
	})
}
