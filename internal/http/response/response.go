package response

import "github.com/gin-gonic/gin"

type Envelope struct {
	Data  any       `json:"data,omitempty"`
	Error *APIError `json:"error,omitempty"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func WriteData(c *gin.Context, status int, data any) {
	c.JSON(
		status,
		Envelope{
			Data: data,
		},
	)
}

func WriteError(c *gin.Context, status int, code string, message string) {
	c.JSON(
		status,
		Envelope{
			Error: &APIError{
				Code:    code,
				Message: message,
			},
		},
	)
}
