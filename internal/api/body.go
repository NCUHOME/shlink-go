package api

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/hakimyu/shlink-go/internal/problem"
)

func DecodeJSON(c *gin.Context, dst any) *problem.Problem {
	if err := c.ShouldBindJSON(dst); err != nil {
		return problem.MalformedBody()
	}
	return nil
}

func DecodeJSONBytes(data []byte, dst any) *problem.Problem {
	if len(bytes.TrimSpace(data)) == 0 {
		return problem.MalformedBody()
	}
	if err := json.Unmarshal(data, dst); err != nil {
		var syntax *json.SyntaxError
		if errors.As(err, &syntax) {
			return problem.MalformedBody()
		}
		return problem.InvalidData()
	}
	return nil
}
