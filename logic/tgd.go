package logic

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"tgautodown/cmd/tg"
	"tgautodown/internal/logs"
)

var Tgs *tg.TgSuber

var namesMap = map[tg.TgMsgClass][]string{
	tg.TgVideo:    {"videos", "视频"},
	tg.TgAudio:    {"music", "音乐"},
	tg.TgDocument: {"documents", "文档"},
	tg.TgPhoto:    {"photos", "照片"},
	tg.TgNote:     {"note", "笔记"},
	"bt":          {"bt", "BT"},
}

func TgSuberStart() {
	Tgs = tg.NewTG(TGCfg.AppID, TGCfg.AppHash, TGCfg.Phone).
		WithSocks5Proxy(TGCfg.socks5).
		WithRetryRule(TGCfg.maxSaveRetryCnt, TGCfg.maxSaveRetrySecond).
		WithSession(TGCfg.sessionPath, TGCfg.f2apwd, waitLoginCode)
		// WithHistoryMsgCnt(16).

	Tgs.WithMsgHandle(tg.TgAudio, func(msgid int, tgmsg *tg.TgMsg) error {
		return doDownload(Tgs, tg.TgAudio, msgid, tgmsg)
	})
	Tgs.WithMsgHandle(tg.TgDocument, func(msgid int, tgmsg *tg.TgMsg) error {
		return doDownload(Tgs, tg.TgDocument, msgid, tgmsg)
	})
	Tgs.WithMsgHandle(tg.TgVideo, func(msgid int, tgmsg *tg.TgMsg) error {
		return doDownload(Tgs, tg.TgVideo, msgid, tgmsg)
	})
	Tgs.WithMsgHandle(tg.TgPhoto, func(msgid int, tgmsg *tg.TgMsg) error {
		return doDownload(Tgs, tg.TgPhoto, msgid, tgmsg)
	})
	Tgs.WithMsgHandle(tg.TgNote, func(msgid int, tgmsg *tg.TgMsg) error {
		if handleDownloadCommand(Tgs, tgmsg) {
			return nil
		}
		if strings.HasPrefix(strings.ToLower(tgmsg.Text), "magnet:?") {
			return downloadMagnet(Tgs, "bt", msgid, tgmsg)
		} else {
			return writeNote(Tgs, tg.TgNote, msgid, tgmsg)
		}
	})

	Tgs.Run(TGCfg.channelNames)
}

var codech chan string

func init() {
	codech = make(chan string)
}

func waitLoginCode() string {
	logs.Info().Msg("waiting for login.code...")
	return <-codech
}
func InputLoginCode(code string) {
	logs.Info().Str("login.code", code).Msg("input")
	codech <- code
}

func onDownloadDone(ts *tg.TgSuber, savePath string, msgid int, tgmsg *tg.TgMsg, err error) {
	var replyMsg string
	if err != nil {
		if errors.Is(err, tg.ErrDownloadDeleted) {
			replyMsg = fmt.Sprintf("下载已删除: %s\n- UID: %d\n- 消息ID: %d",
				tgmsg.FileName, tgmsg.DownloadUID, msgid)
		} else {
			replyMsg = fmt.Sprintf("下载失败: %s\n- UID: %d\n- 消息ID: %d\n- 失败原因: %s",
				tgmsg.FileName, tgmsg.DownloadUID, msgid, err.Error())
		}
	} else {
		replyMsg = fmt.Sprintf("下载成功: %s\n- UID: %d\n- 消息ID: %d\n- 保存路径: %s",
			tgmsg.FileName, tgmsg.DownloadUID, msgid, savePath)
	}
	logs.Debug().Str("from", tgmsg.From.Title).Msg(replyMsg)
	ts.ReplyTo(tgmsg, replyMsg)
}

func doDownload(ts *tg.TgSuber, mtype tg.TgMsgClass, msgid int, tgmsg *tg.TgMsg) error {
	subDir := namesMap[mtype][0]
	mtDesc := namesMap[mtype][1]

	savePath := getSavePath(subDir, tgmsg.FileName)
	if err := ts.SaveFile(tgmsg, savePath, onDownloadDone); err != nil {
		replyMsg := fmt.Sprintf("下载失败: %s\n- 消息ID: %d\n- 失败原因: %s",
			tgmsg.FileName, msgid, err.Error())
		logs.Debug().Str("from", tgmsg.From.Title).Msg(replyMsg)
		return ts.ReplyTo(tgmsg, replyMsg)
	}

	replyMsg := fmt.Sprintf("正在下载%s: %s\n- UID: %d\n- 文件大小: %s\n- 消息ID: %d\n- 暂停请输入: 暂停 %d\n- 删除正在下载视频请输入: 删除 %d\n- 继续请输入: 继续 %d",
		mtDesc, tgmsg.FileName, tgmsg.DownloadUID, sizeInt2Readable(tgmsg.FileSize), msgid,
		tgmsg.DownloadUID, tgmsg.DownloadUID, tgmsg.DownloadUID)
	logs.Debug().Msg(replyMsg)
	ts.ReplyTo(tgmsg, replyMsg)
	return nil
}

func handleDownloadCommand(ts *tg.TgSuber, tgmsg *tg.TgMsg) bool {
	text := strings.TrimSpace(tgmsg.Text)
	cmds := []string{"暂停", "删除", "继续"}

	for _, cmd := range cmds {
		if !strings.HasPrefix(text, cmd) {
			continue
		}

		uidText := strings.TrimSpace(strings.TrimPrefix(text, cmd))
		uid, err := strconv.ParseInt(uidText, 10, 64)
		if err != nil || uid <= 0 {
			ts.ReplyTo(tgmsg, fmt.Sprintf("指令格式错误，请输入：%s UID", cmd))
			return true
		}

		var actionErr error
		var okMsg string
		switch cmd {
		case "暂停":
			actionErr = ts.PauseDownload(uid)
			okMsg = fmt.Sprintf("已暂停下载\n- UID: %d", uid)
		case "删除":
			actionErr = ts.DeleteDownload(uid)
			okMsg = fmt.Sprintf("已删除下载\n- UID: %d", uid)
		case "继续":
			actionErr = ts.ResumeDownload(uid)
			okMsg = fmt.Sprintf("已继续下载\n- UID: %d", uid)
		}

		if actionErr != nil {
			ts.ReplyTo(tgmsg, fmt.Sprintf("操作失败\n- UID: %d\n- 原因: %s", uid, actionErr.Error()))
		} else {
			ts.ReplyTo(tgmsg, okMsg)
		}
		return true
	}
	return false
}

func getSavePath(mtype, filename string) string {
	savePath := filepath.Join(TGCfg.SaveDir, mtype)
	createDir(savePath)
	return filepath.Join(savePath, filename)
	// return uniquePath(savePath)
}

func downloadMagnet(ts *tg.TgSuber, mtype tg.TgMsgClass, msgid int, tgmsg *tg.TgMsg) error {
	subDir := namesMap[mtype][0]
	mtDesc := namesMap[mtype][1]
	url := tgmsg.Text
	logs.Debug().Int("msgid", msgid).Str("url", url).Str("from", tgmsg.From.Title).Msg("recv magnet")

	replyMsg := fmt.Sprintf("正在下载%s:\n- 消息ID: %d", mtDesc, msgid)
	ts.ReplyTo(tgmsg, replyMsg)

	savePath := filepath.Join(TGCfg.SaveDir, subDir)
	createDir(savePath)

	err := exec.Command(TGCfg.Gopeed, "-C", "32", "-D", savePath, url).Run()
	if err != nil {
		replyMsg = fmt.Sprintf("%s下载失败:\n- 消息ID: %d\n- 失败原因: %s",
			mtDesc, msgid, err.Error())
	} else {
		replyMsg = fmt.Sprintf("%s下载成功:\n- 消息ID: %d\n- 保存路径: %s",
			mtDesc, msgid, savePath)
	}
	logs.Debug().Msg(replyMsg)
	return ts.ReplyTo(tgmsg, replyMsg)
}

func writeNote(ts *tg.TgSuber, mtype tg.TgMsgClass, msgid int, tgmsg *tg.TgMsg) error {
	subDir := namesMap[mtype][0]
	mtDesc := namesMap[mtype][1]
	note := tgmsg.Text
	logs.Debug().Int("msgid", msgid).Str("note", note).Str("from", tgmsg.From.Title).Msg("recv note")

	savePath := filepath.Join(TGCfg.SaveDir, subDir)
	createDir(savePath)
	savePath = filepath.Join(savePath, strconv.FormatInt(int64(msgid), 10)+".md")

	err := os.WriteFile(savePath, []byte(note), 0666)
	replyMsg := ""
	if err != nil {
		replyMsg = fmt.Sprintf("%s添加失败:\n- 消息ID: %d\n- 失败原因: %s",
			mtDesc, msgid, err.Error())
	} else {
		replyMsg = fmt.Sprintf("%s添加成功:\n- 消息ID: %d\n- 保存路径: %s",
			mtDesc, msgid, savePath)
	}
	logs.Debug().Msg(replyMsg)
	return ts.ReplyTo(tgmsg, replyMsg)
}

func createDir(dir string) error {
	if fs, err := os.Stat(dir); err == nil {
		if fs.IsDir() {
			return nil
		}
		return fmt.Errorf("same file had been existed")
	}

	return os.MkdirAll(dir, 0777)
}

func sizeInt2Readable(size int64) string {
	if (size >> 30) > 0 {
		return fmt.Sprintf("%.2fGB", float64(size)/1073741824.0)
	}
	if (size >> 20) > 0 {
		return fmt.Sprintf("%.2fMB", float64(size)/1048576.0)
	}
	if (size >> 10) > 0 {
		return fmt.Sprintf("%.2fKB", float64(size)/1024.0)
	}
	return fmt.Sprintf("%d Bytes", size)
}

// 防止重名
func uniquePath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	ext := filepath.Ext(path)
	name := path[:len(path)-len(ext)]
	for i := 1; ; i++ {
		newPath := fmt.Sprintf("%s_%d%s", name, i, ext)
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			return newPath
		}
	}
}
