package telegram

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/perfect-panel/server/internal/config"
	"github.com/perfect-panel/server/internal/model/user"
	"github.com/perfect-panel/server/internal/svc"
	"github.com/perfect-panel/server/pkg/logger"
	"github.com/perfect-panel/server/pkg/tool"
	"github.com/perfect-panel/server/pkg/xerr"
	"github.com/perfect-panel/server/pkg/traffic"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type TelegramLogic struct {
	logger.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTelegramLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TelegramLogic {
	return &TelegramLogic{
		Logger: logger.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TelegramLogic) TelegramLogic(req *tgbotapi.Update) {
	if req.Message != nil && req.Message.Text != "" {
		switch req.Message.Command() {
		case "traffic":
			if err := l.traffic(req.Message.Chat.ID); err != nil {
				l.Logger.Error("[TelegramLogic] Traffic Error: ", logger.Field("error", err.Error()), logger.Field("command", req.Message.Command()), logger.Field("chat_id", req.Message.Chat.ID))
			}
		case "bind":
			if err := l.bind(req.Message.Chat.ID, req.Message.CommandArguments()); err != nil {
				l.Logger.Error("[TelegramLogic] Bind Error: ", logger.Field("error", err.Error()), logger.Field("command", req.Message.Command()), logger.Field("chat_id", req.Message.Chat.ID))
			}
		case "start":
			if err := l.start(req); err != nil {
				l.Logger.Error("[TelegramLogic] Start Error: ", logger.Field("error", err.Error()), logger.Field("command", req.Message.Command()), logger.Field("chat_id", req.Message.Chat.ID), logger.Field("text", req.Message.Text))
			}
		}
	} else {
		l.Logger.Error("[TelegramLogic] Message is empty")
	}
}

func (l *TelegramLogic) sendMessage(bot *tgbotapi.BotAPI, message string, userId int64) error {
	msg := tgbotapi.NewMessage(userId, message)
	msg.ParseMode = "Markdown"
	_, err := bot.Send(msg)
	return err
}

// traffic 查询当前绑定账号的流量使用情况
func (l *TelegramLogic) traffic(chatId int64) error {
	authMethod, err := l.svcCtx.UserModel.FindUserAuthMethodByOpenID(l.ctx, "telegram", strconv.FormatInt(chatId, 10))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return l.sendMessage(l.svcCtx.TelegramBot, TrafficNotBound, chatId)
		}
		l.Errorw("[TelegramLogic] traffic FindUserAuthMethodByOpenID Error", logger.Field("error", err.Error()), logger.Field("chat_id", chatId))
		return l.sendMessage(l.svcCtx.TelegramBot, TrafficQueryFailed, chatId)
	}

	subscribes, err := l.svcCtx.UserModel.QueryUserSubscribe(l.ctx, authMethod.UserId, 1)
	if err != nil {
		l.Errorw("[TelegramLogic] traffic QueryUserSubscribe Error", logger.Field("error", err.Error()), logger.Field("user_id", authMethod.UserId))
		return l.sendMessage(l.svcCtx.TelegramBot, TrafficQueryFailed, chatId)
	}

	if len(subscribes) == 0 {
		return l.sendMessage(l.svcCtx.TelegramBot, TrafficNoSubscribe, chatId)
	}

	var lines string
	for _, sub := range subscribes {
		name := fmt.Sprintf("订阅 #%d", sub.Id)
		if sub.Subscribe != nil && sub.Subscribe.Name != "" {
			name = sub.Subscribe.Name
		}

		used := sub.Download + sub.Upload
		total := sub.Traffic
		remaining := total - used
		if remaining < 0 {
			remaining = 0
		}

		expireStr := sub.ExpireTime.Format("2006-01-02")
		if sub.ExpireTime.IsZero() {
			expireStr = "永久"
		}

		usedStr := traffic.AutoConvert(used, true)
		totalStr := traffic.AutoConvert(total, true)
		remainingStr := traffic.AutoConvert(remaining, true)

		var usagePercent float64
		if total > 0 {
			usagePercent = float64(used) / float64(total) * 100
		}
		progressBar := buildProgressBar(usagePercent)

		lines += fmt.Sprintf(
			"\n📦 *%s*\n"+
				"━━━━━━━━━━━━\n"+
				"📊 已用: `%s` / `%s`\n"+
				"💾 剩余: `%s`\n"+
				"%s %.1f%%\n"+
				"⏰ 到期: `%s`\n",
			escapeMarkdown(name),
			usedStr, totalStr,
			remainingStr,
			progressBar, usagePercent,
			expireStr,
		)
	}

	text, err := tool.RenderTemplateToString(TrafficInfo, map[string]string{
		"Lines": lines,
		"Time":  time.Now().Format("2006-01-02 15:04:05"),
	})
	if err != nil {
		return l.sendMessage(l.svcCtx.TelegramBot, TrafficQueryFailed, chatId)
	}

	return l.sendMessage(l.svcCtx.TelegramBot, text, chatId)
}

// bind 通过订阅链接或裸 token 绑定账号
// 用法1：/bind https://example.com/api/subscribe?token=xxxxxx
// 用法2：/bind xxxxxx（裸 token）
func (l *TelegramLogic) bind(chatId int64, arg string) error {
	if arg == "" {
		return l.sendMessage(l.svcCtx.TelegramBot, BindUsage, chatId)
	}

	// 提取 token：支持完整 URL 和裸 token 两种形式
	token := extractToken(arg)
	if token == "" {
		return l.sendMessage(l.svcCtx.TelegramBot, BindUsage, chatId)
	}

	// 通过订阅 token 查找用户订阅记录
	sub, err := l.svcCtx.UserModel.FindOneSubscribeByToken(l.ctx, token)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return l.sendMessage(l.svcCtx.TelegramBot, BindTokenInvalid, chatId)
		}
		l.Errorw("[TelegramLogic] bind FindOneSubscribeByToken Error", logger.Field("error", err.Error()), logger.Field("token", token))
		return l.sendMessage(l.svcCtx.TelegramBot, BindFailed, chatId)
	}

	userId := sub.UserId

	// 检查该 Telegram 是否已绑定其他账号
	existMethod, err := l.svcCtx.UserModel.FindUserAuthMethodByOpenID(l.ctx, "telegram", strconv.FormatInt(chatId, 10))
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		l.Errorw("[TelegramLogic] bind FindUserAuthMethodByOpenID Error", logger.Field("error", err.Error()))
		return l.sendMessage(l.svcCtx.TelegramBot, BindFailed, chatId)
	}
	if existMethod != nil && existMethod.UserId != 0 && existMethod.UserId != userId {
		return l.sendMessage(l.svcCtx.TelegramBot, BindAlreadyBound, chatId)
	}

	// 查找用户当前是否已有 telegram 绑定
	method, err := l.svcCtx.UserModel.FindUserAuthMethodByPlatform(l.ctx, userId, "telegram")
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		l.Errorw("[TelegramLogic] bind FindUserAuthMethodByPlatform Error", logger.Field("error", err.Error()), logger.Field("userId", userId))
		return l.sendMessage(l.svcCtx.TelegramBot, BindFailed, chatId)
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := l.svcCtx.UserModel.InsertUserAuthMethods(l.ctx, &user.AuthMethods{
			UserId:         userId,
			AuthType:       "telegram",
			AuthIdentifier: strconv.FormatInt(chatId, 10),
			Verified:       true,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}); err != nil {
			l.Errorw("[TelegramLogic] bind InsertUserAuthMethods Error", logger.Field("error", err.Error()), logger.Field("userId", userId))
			return l.sendMessage(l.svcCtx.TelegramBot, BindFailed, chatId)
		}
	} else {
		method.AuthIdentifier = strconv.FormatInt(chatId, 10)
		method.UpdatedAt = time.Now()
		if err := l.svcCtx.UserModel.UpdateUserAuthMethods(l.ctx, method); err != nil {
			l.Errorw("[TelegramLogic] bind UpdateUserAuthMethods Error", logger.Field("error", err.Error()), logger.Field("userId", userId))
			return l.sendMessage(l.svcCtx.TelegramBot, BindFailed, chatId)
		}
	}

	if err := l.svcCtx.UserModel.UpdateUserCache(l.ctx, &user.User{Id: userId}); err != nil {
		l.Errorw("[TelegramLogic] bind UpdateUserCache Error", logger.Field("error", err.Error()), logger.Field("userId", userId))
	}

	text, err := tool.RenderTemplateToString(BindNotify, map[string]string{
		"Id":   strconv.FormatInt(userId, 10),
		"Time": time.Now().Format("2006-01-02 15:04:05"),
	})
	if err != nil {
		return errors.Wrapf(xerr.NewErrCode(xerr.ERROR), "render template failed")
	}
	return l.sendMessage(l.svcCtx.TelegramBot, text, chatId)
}

// extractToken 从完整 URL 或裸 token 中提取 token 值
// 支持：
//   - https://example.com/api/subscribe?token=xxxxxx
//   - xxxxxx（裸 token）
func extractToken(arg string) string {
	u, err := url.Parse(arg)
	if err == nil && u.Scheme != "" && u.Host != "" {
		return u.Query().Get("token")
	}
	return arg
}

func (l *TelegramLogic) start(req *tgbotapi.Update) error {
	if req.Message.CommandArguments() == "" {
		return l.sendMessage(l.svcCtx.TelegramBot, "Please bind account!", req.Message.Chat.ID)
	}
	sessionId := req.Message.CommandArguments()
	sessionIdCacheKey := fmt.Sprintf("%v:%v", config.SessionIdKey, sessionId)
	value, err := l.svcCtx.Redis.Get(context.Background(), sessionIdCacheKey).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		l.Errorw("TelegramLogic start Redis Get Error: ", logger.Field("error", err.Error()), logger.Field("session", sessionId))
		return l.sendMessage(l.svcCtx.TelegramBot, "Bind failed!", req.Message.Chat.ID)
	}
	if value == "" {
		l.Errorw("TelegramLogic start Redis Get Error: ", logger.Field("error", "session not found"), logger.Field("session", sessionId))
		return l.sendMessage(l.svcCtx.TelegramBot, "Bind failed!", req.Message.Chat.ID)
	}
	userId, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		l.Errorw("TelegramLogic start ParseInt Error: ", logger.Field("error", err.Error()), logger.Field("session", sessionId))
		return l.sendMessage(l.svcCtx.TelegramBot, "Bind failed!", req.Message.Chat.ID)
	}

	method, err := l.svcCtx.UserModel.FindUserAuthMethodByPlatform(l.ctx, userId, "telegram")
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		l.Errorw("TelegramLogic start FindUserAuthMethodByPlatform Error: ", logger.Field("error", err.Error()), logger.Field("userId", userId))
		return l.sendMessage(l.svcCtx.TelegramBot, "Bind failed!", req.Message.Chat.ID)
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := l.svcCtx.UserModel.InsertUserAuthMethods(l.ctx, &user.AuthMethods{
			UserId:         userId,
			AuthType:       "telegram",
			AuthIdentifier: strconv.FormatInt(req.Message.Chat.ID, 10),
			Verified:       true,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}); err != nil {
			l.Errorw("TelegramLogic start InsertUserAuthMethod Error: ", logger.Field("error", err.Error()), logger.Field("userId", userId))
			return l.sendMessage(l.svcCtx.TelegramBot, "Bind failed!", req.Message.Chat.ID)
		}
	} else {
		method.AuthIdentifier = strconv.FormatInt(req.Message.Chat.ID, 10)
		if err := l.svcCtx.UserModel.UpdateUserAuthMethods(l.ctx, method); err != nil {
			l.Errorw("TelegramLogic start UpdateUserAuthMethod Error: ", logger.Field("error", err.Error()), logger.Field("userId", userId))
			return l.sendMessage(l.svcCtx.TelegramBot, "Bind failed!", req.Message.Chat.ID)
		}
	}
	if err := l.svcCtx.UserModel.UpdateUserCache(l.ctx, &user.User{Id: userId}); err != nil {
		return errors.Wrapf(xerr.NewErrCode(xerr.ERROR), "update user cache failed")
	}

	text, err := tool.RenderTemplateToString(BindNotify, map[string]string{
		"Id":   strconv.FormatInt(userId, 10),
		"Time": time.Now().Format("2006-01-02 15:04:05"),
	})
	if err != nil {
		return errors.Wrapf(xerr.NewErrCode(xerr.ERROR), "render template failed")
	}
	return l.sendMessage(l.svcCtx.TelegramBot, text, req.Message.Chat.ID)
}

// buildProgressBar 生成 10 格进度条
func buildProgressBar(percent float64) string {
	total := 10
	filled := int(percent / 10)
	if filled > total {
		filled = total
	}
	bar := "["
	for i := 0; i < total; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}
	bar += "]"
	return bar
}

// escapeMarkdown 转义 Markdown 特殊字符
func escapeMarkdown(s string) string {
	specials := []string{"_", "*", "[", "`"}
	for _, c := range specials {
		result := ""
		for i := 0; i < len(s); {
			if i+len(c) <= len(s) && s[i:i+len(c)] == c {
				result += "\\" + c
				i += len(c)
			} else {
				result += string(s[i])
				i++
			}
		}
		s = result
	}
	return s
}