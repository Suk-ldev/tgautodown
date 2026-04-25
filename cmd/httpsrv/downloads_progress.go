package httpsrv

import (
	"encoding/json"
	"net/http"
	"tgautodown/cmd/tg"
	"tgautodown/internal/logs"
	"tgautodown/logic"

	"github.com/oklog/ulid/v2"
)

type DownloadsProgressResponse struct {
	Rtn       int                   `json:"rtn"`
	Msg       string                `json:"msg"`
	Downloads []tg.DownloadSnapshot `json:"downloads"`
}

func HandleDownloadsProgress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rid := ulid.Make().String()
	logs.Info().Rid(rid).Str("path", r.URL.Path).Msg("Downloads progress request")

	resp := DownloadsProgressResponse{
		Rtn: 0,
		Msg: "succ",
	}
	if logic.Tgs != nil {
		resp.Downloads = logic.Tgs.DownloadSnapshots()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logs.Warn(err).Rid(rid).Msg("Failed to encode downloads progress response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
