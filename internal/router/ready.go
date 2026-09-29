package router

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/magicyuan876/yuheng/internal/database"
	"github.com/magicyuan876/yuheng/internal/logger"
)

// readyProbeTimeout bounds each dependency check, so a hung database turns
// /ready into a fast 503 rather than a hung probe.
const readyProbeTimeout = 2 * time.Second

// readyHandler serves /ready, the readiness probe: 200 only when this instance
// can do useful work — the database answers, Redis answers when one is
// configured, and the schema is in a state the code expects (the last
// migration run succeeded and left no dirty version). Anything else is a 503
// with the name of each failing check and no error text, since the endpoint is
// unauthenticated; the reason goes to the log.
//
// /health stays a pure liveness signal ("the process is up"): tying a restart
// to a database outage would only turn a dependency incident into a restart
// loop. Orchestrators route traffic on /ready and restart on /health.
func readyHandler(db *gorm.DB, rdb *redis.Client, migrationReady func() (bool, string)) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), readyProbeTimeout)
		defer cancel()

		checks := gin.H{"database": "ok", "redis": "disabled", "migrations": "ok"}
		ready := true
		fail := func(name string, err error) {
			ready = false
			checks[name] = "failed"
			logger.Warnf(ctx, "[ready] %s check failed: %v", name, err)
		}

		if err := pingDB(ctx, db); err != nil {
			fail("database", err)
		}
		if rdb != nil {
			checks["redis"] = "ok"
			if err := rdb.Ping(ctx).Err(); err != nil {
				fail("redis", err)
			}
		}
		if ok, reason := migrationReady(); !ok {
			ready = false
			checks["migrations"] = "failed"
			logger.Warnf(ctx, "[ready] migrations check failed: %s", reason)
		}

		if !ready {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "checks": checks})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "checks": checks})
	}
}

func pingDB(ctx context.Context, db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// defaultMigrationReady reads the state the start-up migration run recorded.
func defaultMigrationReady() (bool, string) { return database.MigrationReady() }
