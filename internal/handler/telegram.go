package handler

import (
	"github.com/gin-gonic/gin"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/perfect-panel/server/internal/logic/telegram"
	"github.com/perfect-panel/server/internal/svc"
	"github.com/perfect-panel/server/pkg/logger"
	"github.com/perfect-panel/server/pkg/result"
	"github.com/perfect-panel/server/pkg/tool"
)

func RegisterTelegramHandlers(router *gin.Engine, serverCtx *svc.ServiceContext) {
	router.POST("/v1/telegram/webhook", TelegramHandler(serverCtx))
}

func TelegramHandler(svcCtx *svc.ServiceContext) func(c *gin.Context) {
    return func(c *gin.Context) {
        secret := c.Query("secret")

        // 从数据库读 token，和 SetWebhook 时保持同一来源
        tgCfg, err := telegram.GetTelegramConfig(c, svcCtx)
        if err != nil || tgCfg.TelegramBotToken == "" {
            logger.WithContext(c.Request.Context()).Error("[TelegramHandler] Get telegram config failed", logger.Field("error", err))
            c.Abort()
            result.HttpResult(c, nil, nil)
            return
        }

        expected := tool.Md5Encode(tgCfg.TelegramBotToken, false)
        if secret != expected {
            logger.WithContext(c.Request.Context()).Error("[TelegramHandler] Secret mismatch",
                logger.Field("request_secret", secret),
                logger.Field("expected_secret", expected),
            )
            c.Abort()
            result.HttpResult(c, nil, nil)
            return
        }

        var request tgbotapi.Update
        if err := c.BindJSON(&request); err != nil {
            logger.WithContext(c.Request.Context()).Error("[TelegramHandler] Failed to bind request", logger.Field("error", err.Error()))
            c.Abort()
            result.HttpResult(c, nil, err)
            return
        }
        l := telegram.NewTelegramLogic(c, svcCtx)
        l.TelegramLogic(&request)
    }
}
