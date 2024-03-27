package nft

import (
	"encoding/json"
	"io"
	"math/rand"

	"net/http"
	"net/url"
	"time"
	"vdo-platform/internal/dto"
)

type FileHash string

func (t FileHash) GetHash() string {
	return string(t)[len(t)-8:]
}

func (t FileHash) Handle(item *ListenItem) *ListenItem {
	fileMeta, err := getFileStatus(string(t))
	interval := (item.Count/10 + 1) * LISTEN_TIME_INTERVAL_SECOND
	item.Timer = time.NewTimer(time.Duration(interval) * time.Second)
	item.Count++
	if err != nil {
		logger.Error(err, "[Listen file status] get file status error")
		return item
	}
	UpdateVideoStatus(string(t), fileMeta.State)
	if fileMeta.State != FILE_STATUS_ACTIVE &&
		fileMeta.State != FILE_STATUS_CANCEL {
		return item
	}
	logger.Info("[File status update] data hash: %s exit async listening", t.GetHash())
	return nil
}

var fileStateQueryUrl string

func getFileStatus(filehash string) (dto.FileMeta, error) {
	var resp dto.QueryResponse
	url, _ := url.JoinPath(fileStateQueryUrl, filehash)
	response, err := http.Get(url)
	if err != nil {
		return resp.Ok, err
	}
	bytes, err := io.ReadAll(response.Body)
	if err != nil {
		return resp.Ok, err
	}
	err = json.Unmarshal(bytes, &resp)
	return resp.Ok, err
}

var (
	_ ListenedData = FileHash("")
	_ ListenedData = &MockedFileStateHandler{fileHash: ""}
)

type MockedFileStateHandler struct {
	fileHash string
	waitSecs int
}

func NewMockFileStateHandler(fileHash string) *MockedFileStateHandler {
	return &MockedFileStateHandler{fileHash: fileHash, waitSecs: rand.Intn(5) + 1}
}

func (t *MockedFileStateHandler) GetHash() string {
	return t.fileHash[len(t.fileHash)-8:]
}

func (t *MockedFileStateHandler) Handle(item *ListenItem) *ListenItem {
	if item.Count <= 0 {
		item.Timer = time.NewTimer(time.Duration(t.waitSecs) * time.Second)
		item.Count++
		return item
	}
	UpdateVideoStatus(t.fileHash, FILE_STATUS_ACTIVE)
	return nil
}
