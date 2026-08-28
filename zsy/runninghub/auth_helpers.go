package runninghub

import (
	"net/http"

	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

// userAuthRequired delegates to the existing core middleware which operates on
// the standard JWT / session claims attached to the gin context.
func userAuthRequired(c *gin.Context) { middleware.UserAuth()(c) }

func adminAuthRequired(c *gin.Context) { middleware.AdminAuth()(c) }

// ensure both branches compile even when the caller drops them.
var _ = http.StatusUnauthorized
