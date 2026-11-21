package tg

import (
	"context"
	"strconv"
	"tgautodown/internal/logs"

	"github.com/gotd/td/tg"
)

func (ts *TgSuber) Handle(ctx context.Context, u tg.UpdatesClass) error {
	switch upd := u.(type) {
	case *tg.Updates:
		// 常见容器：Updates 包含多个 UpdateClass
		ts.mergeBotChats(upd.Chats)
		for _, up := range upd.Updates {
			ts.handleSingleUpdate(ctx, up)
		}
	case *tg.UpdatesCombined:
		// UpdatesCombined 同样包含 Updates 列表
		ts.mergeBotChats(upd.Chats)
		for _, up := range upd.Updates {
			ts.handleSingleUpdate(ctx, up)
		}
	case *tg.UpdatesTooLong:
		// 服务器提示更新太多，需要从 getDialogs/getHistory 或者 updates.getDifference 拉取
		logs.Warn(nil).Msg("on UpdatesTooLong, need to resync or fetch dialogs/history")
	case *tg.UpdateShort:
		// UpdateShort 包含一个内嵌的 UpdateClass（可能性较少），尝试向下断言
		if inner := upd.Update; inner != nil {
			ts.handleSingleUpdate(ctx, inner)
		}
	default:
		// 其他容器类型，打印以便调试
		logs.Warn(nil).Str("upd", upd.TypeName()).Msg("unknown update class")
	}
	return nil
}

// mergeBotChats 从 updates 容器里提取频道/群信息，动态补全 scis（bot 模式无法预枚举频道）
func (ts *TgSuber) mergeBotChats(chats []tg.ChatClass) {
	if len(chats) == 0 {
		return
	}
	for _, chat := range chats {
		switch c := chat.(type) {
		case *tg.Channel:
			if !ts.matchBotChannel(c.Title, c.ID) {
				continue
			}
			ts.scisMu.Lock()
			if ts.scis == nil {
				ts.scis = map[int64]SubChannelInfo{}
			}
			if _, ok := ts.scis[c.ID]; !ok {
				ts.scis[c.ID] = SubChannelInfo{
					Name:       c.Username,
					Title:      c.Title,
					ChannelID:  c.ID,
					AccessHash: c.AccessHash,
					chType:     ChChannel,
				}
				logs.Info().Int64("id", c.ID).Str("title", c.Title).Str("username", c.Username).Msg("bot discover channel")
			}
			ts.scisMu.Unlock()
		case *tg.Chat:
			if !ts.matchBotChannel(c.Title, c.ID) {
				continue
			}
			ts.scisMu.Lock()
			if ts.scis == nil {
				ts.scis = map[int64]SubChannelInfo{}
			}
			if _, ok := ts.scis[c.ID]; !ok {
				ts.scis[c.ID] = SubChannelInfo{
					Name:      c.Title,
					Title:     c.Title,
					ChannelID: c.ID,
					chType:    ChGroup,
				}
				logs.Info().Int64("id", c.ID).Str("title", c.Title).Msg("bot discover group")
			}
			ts.scisMu.Unlock()
		}
	}
}

// matchBotChannel 判断频道是否在 bot 监听过滤列表里；空列表表示监听全部
func (ts *TgSuber) matchBotChannel(title string, id int64) bool {
	if len(ts.BotChannels) == 0 {
		return true
	}
	idStr := strconv.FormatInt(id, 10)
	for _, c := range ts.BotChannels {
		if c == title || c == idStr {
			return true
		}
	}
	return false
}

func (ts *TgSuber) handleSingleUpdate(ctx context.Context, up tg.UpdateClass) {
	switch u := up.(type) {
	case *tg.UpdateNewMessage:
		if msg, ok := u.Message.(*tg.Message); ok {
			if sci := ts.getSubChannel(msg.PeerID); sci != nil {
				ts.recvChannelMsgHandle(ctx, msg, sci)
			}
		}
	case *tg.UpdateNewChannelMessage:
		if msg, ok := u.Message.(*tg.Message); ok {
			if sci := ts.getSubChannel(msg.PeerID); sci != nil {
				ts.recvChannelMsgHandle(ctx, msg, sci)
			}
		}
	case *tg.UpdateUserStatus:
		logs.Trace().Int64("userid", u.UserID).Str("status", u.Status.String()).Msg("on UpdateUserStatus")
	case *tg.UpdateEditChannelMessage:
		if msg, ok := u.Message.(*tg.Message); ok {
			logs.Trace().Int("msgid", msg.ID).Str("msg", msg.Message).Msg("on UpdateEditChannelMessage")
		} else {
			logs.Trace().Str("msg", u.Message.TypeName()).Msg("on UpdateEditChannelMessage")
		}
	case *tg.UpdateChannelMessageViews:
		logs.Trace().Str("msg", u.String()).Msg("on UpdateChannelMessageViews")
	case *tg.UpdateReadChannelInbox:
		logs.Trace().Str("msg", u.String()).Msg("on UpdateReadChannelInbox")
	case *tg.UpdateReadHistoryOutbox:
		logs.Trace().Str("msg", u.String()).Msg("on UpdateReadHistoryOutbox")
	case *tg.UpdateDraftMessage:
		logs.Trace().Str("msg", u.String()).Msg("on UpdateDraftMessage")
	default:
		logs.Trace().Str("update", u.TypeName()).Msg("un-support updater")
	}
}

func (ts *TgSuber) getSubChannel(peer tg.PeerClass) *SubChannelInfo {
	var chid int64 = 0
	switch p := peer.(type) {
	case *tg.PeerChat:
		chid = p.ChatID
	case *tg.PeerChannel:
		chid = p.ChannelID
	case *tg.PeerUser:
		if p.UserID == 777000 { // 官方通知
			return ts.checkAndGetOfficalInfo(p)
		}
		logs.Trace().Int64("userid", p.UserID).Msg("private chat")
		return nil
	default:
		logs.Warn(nil).Str("peer", peer.TypeName()).Msg("unknown peer")
		return nil
	}

	ts.scisMu.Lock()
	defer ts.scisMu.Unlock()
	if ts.scis != nil {
		if sci, ok := ts.scis[chid]; ok {
			return &sci
		}
	}
	return nil
}

func (ts *TgSuber) checkAndGetOfficalInfo(u *tg.PeerUser) *SubChannelInfo {
	userFull, err := ts.client.API().UsersGetFullUser(ts.gctx, &tg.InputUser{
		UserID: u.UserID,
	})
	if err != nil {
		logs.Error(err).Int64("userid", u.UserID).Msg("GetFullUser error")
		return nil
	}

	// userFull 包含完整信息，包括 profile, bio 等
	user, ok := userFull.Users[0].(*tg.User)
	if !ok {
		logs.Warn(nil).Int64("userid", u.UserID).Msg("GetFullUser error")
		return nil
	}

	logs.Info().Int64("userid", user.ID).
		Int64("accesshash", user.AccessHash).
		Str("username", user.Username).
		Str("firstname", user.FirstName).
		Str("lastname", user.LastName).
		Bool("verify", user.Verified).Msg("get user info")

	return &SubChannelInfo{
		AccessHash: user.AccessHash,
		ChannelID:  user.ID,
		Name:       user.LastName,
		Pts:        0,
		Title:      user.FirstName,
		chType:     ChTelegramService,
	}
}
