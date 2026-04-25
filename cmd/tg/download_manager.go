package tg

import (
	"context"
	"errors"
	"os"
	"sync"
	"tgautodown/internal/logs"
	"time"
)

var (
	ErrDownloadNotFound = errors.New("download task not found")
	ErrDownloadDeleted  = errors.New("download task deleted")
)

type DownloadState string

const (
	DownloadQueued      DownloadState = "queued"
	DownloadDownloading DownloadState = "downloading"
	DownloadPaused      DownloadState = "paused"
)

type DownloadSnapshot struct {
	UID        int64  `json:"uid"`
	MsgID      int    `json:"msgid"`
	FileName   string `json:"filename"`
	SavePath   string `json:"savePath"`
	Class      string `json:"class"`
	State      string `json:"state"`
	Downloaded int64  `json:"downloaded"`
	Total      int64  `json:"total"`
	Progress   string `json:"progress"`
	Percent    int    `json:"percent"`
	CreatedAt  int64  `json:"createdAt"`
}

type DownloadTask struct {
	UID       int64
	MsgID     int
	FileName  string
	SavePath  string
	Class     TgMsgClass
	CreatedAt time.Time

	mu         sync.Mutex
	state      DownloadState
	downloaded int64
	total      int64
	paused     bool
	deleted    bool
}

type DownloadManager struct {
	next  int64
	free  []int64
	mu    sync.RWMutex
	tasks map[int64]*DownloadTask
}

func NewDownloadManager() *DownloadManager {
	dm := &DownloadManager{
		tasks: map[int64]*DownloadTask{},
		next:  99,
	}
	return dm
}

func (dm *DownloadManager) Add(tgmsg *TgMsg, savePath string) *DownloadTask {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	uid := dm.allocUIDLocked()
	task := &DownloadTask{
		UID:       uid,
		MsgID:     tgmsg.msg.ID,
		FileName:  tgmsg.FileName,
		SavePath:  savePath,
		Class:     tgmsg.mcls,
		CreatedAt: time.Now(),
		state:     DownloadQueued,
		total:     tgmsg.FileSize,
	}

	dm.tasks[uid] = task
	tgmsg.DownloadUID = uid
	logs.Debug().Int64("uid", uid).Int("msgid", tgmsg.msg.ID).Str("filename", tgmsg.FileName).Msg("download uid alloc")
	return task
}

func (dm *DownloadManager) allocUIDLocked() int64 {
	if len(dm.free) == 0 {
		dm.next++
		return dm.next
	}

	minIdx := 0
	for i := 1; i < len(dm.free); i++ {
		if dm.free[i] < dm.free[minIdx] {
			minIdx = i
		}
	}

	uid := dm.free[minIdx]
	dm.free = append(dm.free[:minIdx], dm.free[minIdx+1:]...)
	return uid
}

func (dm *DownloadManager) Release(uid int64) {
	dm.mu.Lock()
	if _, ok := dm.tasks[uid]; ok {
		delete(dm.tasks, uid)
		dm.free = append(dm.free, uid)
		logs.Debug().Int64("uid", uid).Msg("download uid release")
	}
	dm.mu.Unlock()
}

func (dm *DownloadManager) Get(uid int64) (*DownloadTask, bool) {
	dm.mu.RLock()
	task, ok := dm.tasks[uid]
	dm.mu.RUnlock()
	return task, ok
}

func (dm *DownloadManager) Pause(uid int64) error {
	task, ok := dm.Get(uid)
	if !ok {
		return ErrDownloadNotFound
	}
	task.Pause()
	return nil
}

func (dm *DownloadManager) Resume(uid int64) error {
	task, ok := dm.Get(uid)
	if !ok {
		return ErrDownloadNotFound
	}
	task.Resume()
	return nil
}

func (dm *DownloadManager) Delete(uid int64) error {
	task, ok := dm.Get(uid)
	if !ok {
		return ErrDownloadNotFound
	}
	task.Delete()
	dm.Release(uid)
	_ = os.Remove(task.SavePath + ".dl")
	_ = os.Remove(task.SavePath)
	return nil
}

func (dm *DownloadManager) Snapshots() []DownloadSnapshot {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	snapshots := make([]DownloadSnapshot, 0, len(dm.tasks))
	for _, task := range dm.tasks {
		snapshots = append(snapshots, task.Snapshot())
	}
	return snapshots
}

func (t *DownloadTask) Pause() {
	t.mu.Lock()
	if !t.deleted {
		t.paused = true
		t.state = DownloadPaused
	}
	t.mu.Unlock()
}

func (t *DownloadTask) Resume() {
	t.mu.Lock()
	if !t.deleted {
		t.paused = false
		t.state = DownloadDownloading
	}
	t.mu.Unlock()
}

func (t *DownloadTask) Delete() {
	t.mu.Lock()
	t.deleted = true
	t.paused = false
	t.mu.Unlock()
}

func (t *DownloadTask) IsDeleted() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.deleted
}

func (t *DownloadTask) WaitIfPaused(ctx context.Context) error {
	for {
		t.mu.Lock()
		if t.deleted {
			t.mu.Unlock()
			return ErrDownloadDeleted
		}
		if !t.paused {
			t.mu.Unlock()
			return nil
		}
		t.state = DownloadPaused
		t.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func (t *DownloadTask) SetDownloading() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.deleted {
		return ErrDownloadDeleted
	}
	if !t.paused {
		t.state = DownloadDownloading
	}
	return nil
}

func (t *DownloadTask) SetProgress(downloaded, total int64) {
	t.mu.Lock()
	t.downloaded = downloaded
	t.total = total
	if !t.paused && !t.deleted {
		t.state = DownloadDownloading
	}
	t.mu.Unlock()
}

func (t *DownloadTask) Snapshot() DownloadSnapshot {
	t.mu.Lock()
	defer t.mu.Unlock()

	percent := 0
	if t.total > 0 {
		percent = int(float64(t.downloaded) * 100 / float64(t.total))
		if percent > 100 {
			percent = 100
		}
	}

	return DownloadSnapshot{
		UID:        t.UID,
		MsgID:      t.MsgID,
		FileName:   t.FileName,
		SavePath:   t.SavePath,
		Class:      string(t.Class),
		State:      string(t.state),
		Downloaded: t.downloaded,
		Total:      t.total,
		Progress:   calcDlProgress(t.downloaded, t.total),
		Percent:    percent,
		CreatedAt:  t.CreatedAt.Unix(),
	}
}
