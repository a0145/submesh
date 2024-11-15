package web

import (
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"submesh/submesh/contextkeys"
	"submesh/submesh/state"
	"submesh/submesh/types"
	"time"

	"buf.build/gen/go/meshtastic/protobufs/protocolbuffers/go/meshtastic"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/gomig/avatar"
	"go.uber.org/zap"
)

func truncArray(arr []any, n int) []any {
	if len(arr) > n {
		return arr[:n]
	}
	return arr
}

func formatAsDate(t time.Time) string {
	year, month, day := t.Date()
	return fmt.Sprintf("%d%02d/%02d", year, month, day)
}

func timeAgo(timestamp *uint32) string {
	if timestamp == nil {
		return ""
	}

	now := time.Now().Unix()
	diff := now - int64(*timestamp)

	if diff < 0 {
		return "in the future"
	}

	if diff < 60 {
		return fmt.Sprintf("%d seconds", diff)
	}

	if diff < 3600 {
		return fmt.Sprintf("%d minutes", diff/60)
	}

	if diff < 86400 {
		return fmt.Sprintf("%d hours", diff/3600)
	}

	return fmt.Sprintf("%d days", diff/86400)
}
func timeUptime(timestamp *uint32) string {
	if timestamp == nil {
		return ""
	}

	diff := int64(*timestamp)

	if diff < 0 {
		return "in the future"
	}

	if diff < 60 {
		return fmt.Sprintf("%d seconds", diff)
	}

	if diff < 3600 {
		return fmt.Sprintf("%d minutes", diff/60)
	}

	if diff < 86400 {
		return fmt.Sprintf("%d hours", diff/3600)
	}

	return fmt.Sprintf("%d days", diff/86400)
}

func hexCodeToId(hexCode string) uint32 {
	stripped := strings.TrimPrefix(hexCode, "!")
	decimal_num, _ := strconv.ParseInt(stripped, 16, 64)
	return uint32(decimal_num)
}

func ApiMiddleware(ctx context.Context) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("statedb", ctx.Value(contextkeys.State).(*state.State))
		c.Next()
	}
}

type TwoRow struct {
	Num    int
	First  uint32
	Second string
}

func StartServer(ctx context.Context) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.Use(ApiMiddleware(ctx))
	router.SetFuncMap(template.FuncMap{
		"appVersion": func() string {
			return AppVersion
		},
		"formatAsDate": formatAsDate,
		"timeAgo":      timeAgo,
		"timeUptime":   timeUptime,
		"idToShortaddr": func(id uint32) string {
			//lookup user
			if id == 4294967295 {
				return "Broadcast"
			}
			h := fmt.Sprintf("!%x", id)

			userObj := ctx.Value(contextkeys.State).(*state.State).Users.LastBy(h)
			if userObj != nil {
				return userObj.Underlying.ShortName
			}
			return h
		},
		"bytesToB64String": func(b []byte) string {
			return base64.StdEncoding.EncodeToString(b)
		},
		"yesno": func(b bool) string {
			if b {
				return "Yes"
			}
			return "No"
		},
		"yesnoemoji": func(b bool) string {
			if b {
				return "✅"
			}
			return "❌"
		},
		"arr": func(els ...any) []any {
			return els
		},
		"trunc_arr": func(arr []any, n int) []any {
			if len(arr) > n {
				return arr[:n]
			}
			return arr
		},
		"avatar_for": func(id uint32) template.HTML {
			john := avatar.NewTextAvatar(fmt.Sprintf("%d", id)) // make avatar with J letter
			htmlTag := john.InlineSVG()
			return template.HTML(htmlTag)
		},
		"snrMeter": func(snr float32) template.HTML {
			return template.HTML(fmt.Sprintf(`<div style='text-align:center;'>%0.2f<br><meter value="%f" min="-20" low="-10" optimum="0" max="10"></meter></div>`, snr, snr))
		},
		"toUint32": func(s string) uint32 {
			i, _ := strconv.Atoi(s)
			return uint32(i)
		},
		"prefixedHexIdToUint32": func(s string) uint32 {
			return hexCodeToId(s)
		},
		"emptyNilFloat32": func(s *float32) string {
			if s == nil {
				return ""
			}
			return fmt.Sprintf("%.2f", *s)
		},
		"emptyNilUint32": func(s *uint32) string {
			if s == nil {
				return ""
			}
			return fmt.Sprintf("%d", *s)
		},
		"addUnit": func(s string, unit string) string {
			if s == "" {
				return ""
			}
			return fmt.Sprintf("%s%s", s, unit)
		},
		"coordToFloat": func(s int32) float32 {
			return float32(s) * 1e-7
		},
		"longNameFromId": func(id uint32) string {
			userObj := ctx.Value(contextkeys.State).(*state.State).Users.LastBy(fmt.Sprintf("%d", id))
			if userObj != nil {
				return userObj.Underlying.LongName
			}
			return "unknown"
		},
		"lastHeard": func(id uint32) string {
			userObj := ctx.Value(contextkeys.State).(*state.State).AllMessages.LastByProperty("From", fmt.Sprintf("%d", id))
			if userObj != nil {
				return timeAgo(&userObj.RxTime)
			}
			return "unknown"
		},
		"timeAgoInt": func(id uint32) string {
			return timeAgo(&id)
		},
		"lastAltitide": func(id uint32) string {
			userObj := ctx.Value(contextkeys.State).(*state.State).Positions.LastBy(fmt.Sprintf("%d", id))
			if userObj != nil && userObj.Underlying.Altitude != nil {
				return fmt.Sprintf("%d", *userObj.Underlying.Altitude)
			}
			return "unknown"
		},
		"tracerouteTo": func(route *meshtastic.RouteDiscovery) []TwoRow {
			ret := []TwoRow{}
			for i := 0; i < len(route.Route); i++ {
				snr := "unknown"
				if len(route.SnrTowards)-1 >= i {
					snr = fmt.Sprintf("%d", route.SnrTowards[i])
				}
				ret = append(ret, TwoRow{i, route.Route[i], snr})
			}
			return ret
		},
		"tracerouteFrom": func(route *meshtastic.RouteDiscovery) []TwoRow {
			ret := []TwoRow{}
			for i := 0; i < len(route.RouteBack); i++ {
				snr := "unknown"
				if len(route.SnrBack)-1 >= i {
					snr = fmt.Sprintf("%d", route.SnrBack[i])
				}
				ret = append(ret, TwoRow{i, route.RouteBack[i], snr})
			}
			return ret
		},
	})
	LoadHTMLFromEmbedFS(router, templatesFS, "templates/*.html")

	logger := ctx.Value(contextkeys.Logger).(*zap.Logger)
	router.Use(ginzap.Ginzap(logger, time.RFC3339, true))

	router.Use(ginzap.RecoveryWithZap(logger, true))

	router.GET("/", func(c *gin.Context) {
		sdb, _ := c.MustGet("statedb").(*state.State)

		c.HTML(http.StatusOK, "templates/users.html", gin.H{
			"Users": sdb.Users.OnlyMostRecentByUnderlyingPropertyString("Id"),
		})
	})

	router.GET("/user", func(c *gin.Context) {
		id := c.Query("id")
		sdb, _ := c.MustGet("statedb").(*state.State)
		user := sdb.Users.LastBy(id)
		var intId = uint32(0)
		var position *types.ParsedMessage[meshtastic.Position]
		var telemetry *types.ParsedMessage[meshtastic.Telemetry]
		if user != nil {
			intId = hexCodeToId(user.Underlying.Id)
			position = ctx.Value(contextkeys.State).(*state.State).Positions.LastBy(fmt.Sprintf("%d", intId))
			telemetry = ctx.Value(contextkeys.State).(*state.State).Telemetry.LastBy(fmt.Sprintf("%d", intId))
		}
		limitTo := 500
		from := sdb.AllMessages.FilteredByString("From", fmt.Sprintf("%d", intId))
		if len(from) > limitTo {
			from = from[:limitTo]
		}
		to := sdb.AllMessages.FilteredByString("To", fmt.Sprintf("%d", intId))
		if len(to) > limitTo {
			to = to[:limitTo]
		}
		c.HTML(http.StatusOK, "templates/user.html", gin.H{
			"QueryUser": id,
			"User":      user,
			"Position":  position,
			"Telemetry": telemetry,
			"intId":     intId,
			"FromMsgs":  from,
			"ToMsgs":    to,
		})
	})

	router.GET("/chats", func(c *gin.Context) {
		sdb, _ := c.MustGet("statedb").(*state.State)
		c.HTML(http.StatusOK, "templates/chats.html", gin.H{
			"Chats": sdb.Chats.All(),
		})
	})

	router.GET("/neighbors", func(c *gin.Context) {
		sdb, _ := c.MustGet("statedb").(*state.State)
		c.HTML(http.StatusOK, "templates/neighbors.html", gin.H{
			"Neighbors": sdb.Neighbors.OnlyMostRecentByUnderlyingPropertyString("NodeId"),
		})
	})
	router.GET("/telemetry", func(c *gin.Context) {
		sdb, _ := c.MustGet("statedb").(*state.State)
		limit := 500
		telemetry := sdb.Telemetry.All()
		if len(telemetry) > limit {
			telemetry = telemetry[:limit]
		}
		c.HTML(http.StatusOK, "templates/telemetry.html", gin.H{
			"Telemetry": telemetry,
		})
	})
	router.GET("/traceroutes", func(c *gin.Context) {
		sdb, _ := c.MustGet("statedb").(*state.State)
		limit := 500
		traceroutes := sdb.Traceroutes.All()
		if len(traceroutes) > limit {
			traceroutes = traceroutes[:limit]
		}
		c.HTML(http.StatusOK, "templates/traceroutes.html", gin.H{
			"Traceroutes": traceroutes,
		})
	})
	router.GET("/nondecryptable", func(c *gin.Context) {
		sdb, _ := c.MustGet("statedb").(*state.State)
		c.HTML(http.StatusOK, "templates/nondecryptable.html", gin.H{
			"NonDecryptable": sdb.NonDecryptable.All(),
		})
	})
	router.GET("/map", func(c *gin.Context) {
		sdb, _ := c.MustGet("statedb").(*state.State)
		c.HTML(http.StatusOK, "templates/map.html", gin.H{
			"Positions": sdb.Positions.OnlyMostRecentByPropertyString("From"),
		})
	})
	router.GET("/all", func(c *gin.Context) {
		sdb, _ := c.MustGet("statedb").(*state.State)
		allm := sdb.AllMessages.All()
		only := 500
		if len(allm) > only {
			allm = allm[:only]
		}
		c.HTML(http.StatusOK, "templates/all.html", gin.H{
			"All": allm,
		})
	})
	router.GET("/hi", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "hi",
		})
	})

	router.Run(":8080")

}
