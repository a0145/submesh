package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
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
func longNameFromId(state *state.State, id uint32) string {
	userObj := state.Users.LastBy(fmt.Sprintf("%d", id))
	if userObj != nil {
		return userObj.Underlying.LongName
	}
	return "unknown"
}
func idToShortaddr(state *state.State, id uint32) string {
	//lookup user
	if id == 4294967295 {
		return "Broadcast"
	}
	h := fmt.Sprintf("!%x", id)

	userObj := state.Users.LastBy(h)
	if userObj != nil {
		return userObj.Underlying.ShortName
	}
	return h
}
func lastAltitude(state *state.State, id uint32) string {
	userObj := state.Positions.LastBy(fmt.Sprintf("%d", id))
	if userObj != nil && userObj.Underlying.Altitude != nil {
		return fmt.Sprintf("%d", *userObj.Underlying.Altitude)
	}
	return "unknown"
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

func coordToFloat(s int32) float32 {
	return float32(s) * 1e-7
}

type HeatMapData struct {
	Id            uint32
	Lat           float32
	Long          float32
	Hits          int
	ShortAddr     string
	LongName      string
	LastHeard     string
	LastAltitude  string
	PrecisionBits uint32
}

func lastHeard(state *state.State, id uint32) string {
	userObj := state.AllMessages.LastByProperty("From", fmt.Sprintf("%d", id))
	if userObj != nil {
		return timeAgo(&userObj.RxTime)
	}
	return "unknown"
}

func hitmapToHeatmap(state *state.State, hitMap map[uint32]int) template.JS {
	assembled := []HeatMapData{}

	for peerId, hitCount := range hitMap {
		locationOfPeer := state.Positions.LastBy(fmt.Sprintf("%d", peerId))
		if locationOfPeer != nil && locationOfPeer.Underlying.LatitudeI != nil && locationOfPeer.Underlying.LongitudeI != nil {
			altitude := "unknown"
			if locationOfPeer.Underlying.Altitude != nil {
				altitude = fmt.Sprintf("%d", *locationOfPeer.Underlying.Altitude)
			}
			assembled = append(assembled, HeatMapData{
				Id:            peerId,
				Lat:           coordToFloat(*locationOfPeer.Underlying.LatitudeI),
				Long:          coordToFloat(*locationOfPeer.Underlying.LongitudeI),
				Hits:          hitCount,
				ShortAddr:     idToShortaddr(state, peerId),
				LongName:      longNameFromId(state, peerId),
				LastHeard:     lastHeard(state, peerId),
				LastAltitude:  altitude,
				PrecisionBits: locationOfPeer.Underlying.PrecisionBits,
			})
		}
	}

	marshalled, _ := json.Marshal(assembled)
	return template.JS(string(marshalled))

}

func heatmapMessageCount(state *state.State) template.JS {
	hitMap := map[uint32]int{}

	for _, tr := range state.AllMessages.All() {
		hitMap[tr.From]++
	}

	return hitmapToHeatmap(state, hitMap)
}

func tracerouteHeatmap(state *state.State) template.JS {
	hitMap := map[uint32]int{}
	max := 0

	for _, tr := range state.Traceroutes.All() {
		for _, routingPeer := range tr.Underlying.Route {
			hitMap[routingPeer]++
			if hitMap[routingPeer] > max {
				max = hitMap[routingPeer]
			}
		}
	}

	return hitmapToHeatmap(state, hitMap)
}

type TwoRow struct {
	Num    int
	First  uint32
	Second string
}

func StartServer(ctx context.Context) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	router.Use(ApiMiddleware(ctx))
	router.SetFuncMap(template.FuncMap{
		"appVersion": func() string {
			return ctx.Value(contextkeys.AppVersion).(string)
		},
		"formatAsDate": formatAsDate,
		"timeAgo":      timeAgo,
		"timeUptime":   timeUptime,
		"parseUint32": func(s string) uint32 {
			decimal_num, _ := strconv.ParseInt(s, 10, 64)
			return uint32(decimal_num)
		},
		"idToShortaddr": func(id uint32) string {
			return idToShortaddr(ctx.Value(contextkeys.State).(*state.State), id)
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
		"coordToFloat": coordToFloat,
		"longNameFromId": func(id uint32) string {
			return longNameFromId(ctx.Value(contextkeys.State).(*state.State), id)
		},
		"unixToHourDate": func(t uint32) string {
			return time.Unix(int64(t), 0).Format("02/01/2006 15:04")
		},
		"lastHeard": func(id uint32) string {
			return lastHeard(ctx.Value(contextkeys.State).(*state.State), id)
		},
		"timeAgoInt": func(id uint32) string {
			return timeAgo(&id)
		},
		"lastAltitide": func(id uint32) string {
			return lastAltitude(ctx.Value(contextkeys.State).(*state.State), id)
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

		// assume the query id is a decimal version
		decimal_num, _ := strconv.ParseInt(id, 10, 64)
		var intId = uint32(decimal_num)
		var position *types.ParsedMessage[meshtastic.Position]
		var telemetry *types.ParsedMessage[meshtastic.Telemetry]
		var allTelemetry []types.ParsedMessage[meshtastic.Telemetry]

		if user != nil {
			intId = hexCodeToId(user.Underlying.Id)
			position = ctx.Value(contextkeys.State).(*state.State).Positions.LastBy(fmt.Sprintf("%d", intId))
			telemetry = ctx.Value(contextkeys.State).(*state.State).Telemetry.LastBy(fmt.Sprintf("%d", intId))
			allTelemetry = ctx.Value(contextkeys.State).(*state.State).Telemetry.FilteredByString("From", fmt.Sprintf("%d", intId))
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
		fmt.Println("ASDF: to", to)
		c.HTML(http.StatusOK, "templates/user.html", gin.H{
			"QueryUser":     id,
			"User":          user,
			"Position":      position,
			"LastTelemetry": telemetry,
			"Telemetry":     allTelemetry,
			"intId":         intId,
			"FromMsgs":      from,
			"ToMsgs":        to,
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
			"Heatmap":     tracerouteHeatmap(sdb),
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
			"Heatmap":   heatmapMessageCount(sdb),
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
